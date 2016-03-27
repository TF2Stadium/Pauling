package logs

import (
	"bytes"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"runtime"
	"time"

	"github.com/TF2Stadium/Pauling/config"
)

// success	True / False
// error	Description of error.
// log_id	ID of the log on successful upload.
// url	Relative path to the log. e.g. /5100

type response struct {
	Error   string `json:"error,omitempty"`
	LogID   int    `json:"log_id,omitempty"`
	Success bool   `json:"success"`
}

var client = http.Client{
	Timeout: 10 * time.Second,
}

func Upload(title, mapName string, logs *bytes.Buffer) (int, error) {
	//To upload logs, make a multipart/form-data POST request to http://logs.tf/upload
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	//logfile-Log file. Max 5 MB.
	part, err := writer.CreateFormFile("logfile", "log.log")
	if err != nil {
		return 0, err
	}

	part.Write(logs.Bytes())
	//title-Title of your log. Max length 40 chars.
	writer.WriteField("title", title)
	//map-TF2 map. Optional. Max length 24 chars.
	writer.WriteField("map", mapName)
	//uploader-Optional Name of the uploading plugin or software (including version). Max length 40 chars.
	writer.WriteField("uploader", "TF2Stadium")
	//key-Your unique key, see "Logs.tf API key" on upload page.
	writer.WriteField("key", config.Constants.LogsTFAPIKey)
	writer.Close()

	req, err := http.NewRequest("POST", "http://logs.tf/upload", body)
	req.Header.Add("Content-Type", writer.FormDataContentType())
	req.Header.Add("User-Agent", runtime.Version())

	re, err := client.Do(req)
	if err != nil {
		return 0, err
	}

	resp := response{}
	dec := json.NewDecoder(re.Body)
	dec.Decode(&resp)

	return checkSuccess(resp)
}

func checkSuccess(resp response) (int, error) {
	if !resp.Success {
		return 0, errors.New(resp.Error)
	}

	return resp.LogID, nil
}
