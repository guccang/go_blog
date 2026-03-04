
p=$(dirname $0)

p=$(realpath "$p")
echo $p

sh $p/stop.sh

sh $p/run.sh
