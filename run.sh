#!/usr/bin/env bash

# Enable i2c-1
if ! [ -a /dev/i2c-1 ]; then
    echo "Enabling devices" | tee -a /data/start.log
    echo 'BB-I2C1' > /sys/devices/platform/bone_capemgr/slots
    echo 'BB-UART1' > /sys/devices/platform/bone_capemgr/slots
    echo 'BB-UART2' > /sys/devices/platform/bone_capemgr/slots
fi

# Setup local clock in case we are offline
if ! [ -a /dev/rtc1 ]; then
    # Create RTC device if not existing
    echo ds3231 0x68 >/sys/bus/i2c/devices/i2c-2/new_device
fi
sleep 1
hwclock -f /dev/rtc1 -s

echo "Started at " $(date) | tee -a /data/start.log

# Start application
godynastat
