#!/bin/sh
# Создание необходимых ссылок

ln -s -f admin.sh start
ln -s -f admin.sh exist
ln -s -f admin.sh create
ln -s -f admin.sh logrotate
ln -s -f admin.sh omap
ln -s -f admin.sh msgmap
ln -s -f admin.sh setValue
ln -s -f admin.sh getValue
ln -s -f admin.sh getRawValue
ln -s -f admin.sh getCalibrate
ln -s -f admin.sh help

ln -s -f /usr/bin/uniset-stop.sh stop.sh
ln -f -s ../../../conf/configure.xml
