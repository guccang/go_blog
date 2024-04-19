
p=$(dirname $0)
p=$(realpath "$p")

base_path=$p/redis

cd $base_path
echo "----------------------------------------"
echo $basepath
echo "----------------------------------------"
$base_path/redis-server $base_path/redis_6666.conf
