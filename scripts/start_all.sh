
p=$(dirname $0)

p=$(realpath "$p")

sh $p/start_redis.sh

sh $p/start.sh
