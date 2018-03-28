# configure CANBus interface
/sbin/ip link set can0 type can bitrate 500000 restart-ms 100
/sbin/ifconfig can0 up
/sbin/ifconfig can0
/usr/bin/candump -n 5 can0
