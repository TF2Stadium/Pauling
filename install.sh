#!/bin/sh

go build -v

sudo cp ./Pauling /usr/bin/Pauling
sudo cp ./etc/Pauling.service /usr/lib/systemd/system/

sudo systemctl daemon-reload
