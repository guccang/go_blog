

p=$(dirname $0)
p=$(realpath "$p")
base_path=$p

num=$(ps aux | grep "$base_path" | grep "$base_path/go_blog" | awk '{print $2}'|wc -l)

if [ $num -gt 0 ];then
	ps aux | grep "$base_path" | grep "$base_path/go_blog" | awk '{print $2}' | xargs kill -9
fi

sh $p/show.sh
