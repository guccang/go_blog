
p=$(dirname $0)
p=$(realpath $p)
p=$(dirname $p)
echo $p
ps aux | grep "$p" | grep -v "grep" | grep "redis/redis-server" | awk '{print $2}' | xargs kill -9
