#!/bin/bash

if [ "$1" = "lip" ]; then
	hostname -I
	exit 0
fi
if [ "$1" = "poweroff" ]; then
	sudo /sbin/poweroff
fi

if [ -e "/home/McServer/maint" ]; then
    echo "Sorry, the servers are down for maintenance. Check back later."
    [ -s /home/McServer/maint ] && cat /home/McServer/maint
    exit
fi

if [ "$1" = "help" ]; then
	echo "Usage:"
	echo "~!gsm <server> - Start a server"
	echo "~!gsm stop - Send stop command to all servers"
	echo "~!gsm list - List of servers"
	echo "~!gsm ping - List servers that are up"
	echo "~!gsm ip - Get IP address"
	exit
fi

if [ "$1" = "ip" ]; then
	curl https://checkip.amazonaws.com
	exit 0
fi

if [ "$1" = "update" ]; then
    for dir in /home/McServer/*/; do
        cd $dir
        if [ -x "update.py" ]; then
            ./update.py
        fi
        cd ..
    done
    pip install --user -q -U yt-dlp mcstatus
    exit 0
fi

if [ -z "$1" ] || [ "$1" = "list" ]; then
	echo "List of servers:"
	ls -d /home/McServer/*/ | awk '{split($0,a,"/"); print a[5]}'
	echo "Use ~!gsm <server> to start the server or ~!gsm help for additional commands."
	exit
fi

if [ "$1" = "stop" ]; then
	screen -S McServer -p 0 -X stuff "stop^M" > /dev/null
	screen -S Terraria -p 0 -X stuff "exit^M" > /dev/null
	echo "Sent stop command to server."
    exit
fi
if [ "$1" = "ping" ]; then
    PID=($(screen -list| head -n -1 | tail -n +2 | cut -d'.' -f1 | xargs))
	# screen -list | head -n -1 | tail -n+2 | awk '{print $1}' | cut -d'.' -f2-
	if [ -z "$PID" ]; then
        echo "No servers online."
    else
        echo Servers:
        for i in "${PID[@]}"; do
            lsof -p $i | awk 'NR == 2 {print $9}' | cut -d'/' -f5
        done
	fi
	exit
fi

if [ ! -d "/home/McServer/$1" ]; then
	echo "Invalid server!"
	exit
fi
if [ -e "/home/McServer/$1/maint" ]; then
    echo "Sorry, this instance is undergoing maintenance. Check back later."
    [ -s "/home/McServer/$1/maint" ] && cat "/home/McServer/$1/maint"
    exit
fi
screen -list | grep Terraria > /dev/null && echo "There is already a server running!" && exit
screen -list | grep McServer > /dev/null && echo "There is already a server running!" && exit
cd /home/McServer/$1
if ./bg.sh "$2" > /dev/null 2>&1; then
    echo "$1" "should be up shortly..."
else
    echo "There was an error starting the server."
fi
