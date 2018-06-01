
LGRESU_MON_HOME=/opt/lgresu

cd ${LGRESU_MON_HOME}

mkdir -p log
mkdir -p data

nohup ./bin/lg_resu_mon -if can0 -d info -p 9090 -dr ${LGRESU_MON_HOME}/data -r 7 > ./log/lg_resu_mon.log 2>&1 &


