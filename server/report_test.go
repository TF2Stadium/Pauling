package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func init() {
	CreateDB()
}

func TestNewReport(t *testing.T) {
	t.Parallel()
	var (
		source       = "1"
		target       = "2"
		lobbyID uint = 3
	)

	err := newReport(source, target, lobbyID)
	assert.NoError(t, err)

	assert.True(t, hasReported(source, target, lobbyID))
	assert.Equal(t, countReports(target, lobbyID), 1)

	err = ResetReportCount(target, 3)
	assert.NoError(t, err)
	assert.Zero(t, countReports(target, lobbyID))
	assert.False(t, hasReported(source, target, lobbyID))
}
