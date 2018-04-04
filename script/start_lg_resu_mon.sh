cd /opt/lgresu
mkdir -p log
nohup ./bin/lg_resu_mon -if can0 > ./log/lg_resu_mon.log 2>&1 &


