while sleep 20
do
    echo "keep_alive.sh: sending keep alive message (305): "; date; 
    cansend can0 305#0000000000000000;
done
