p=$(dirname $0)
p=$(realpath "$p")

sh $p/stop.sh
sh $p/stop_redis.sh
sh $p/show.sh
