#!/bin/sh

go build -v

sudo cp ./Pauling /usr/bin/Pauling
sudo cp ./etc/Pauling.service /etc/systemd/user/

sudo systemctl daemon-reload
