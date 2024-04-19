p=$(dirname $0)

p=$(realpath "$p")

base_path=$(dirname $p)

run_path=$1
if [ "$run_path" == "" ]; then
	run_path="$base_path/bin"
fi

if [ ! -d $run_path ];then
	mkdir -p "$run_path"
fi

echo $p
echo run_path=$run_path

# data copy
bin_path=$run_path

mkdir -p $bin_path
mkdir -p $bin_path/blogs_txt
mkdir -p $bin_path/redis

if [ -e $run_path/blog.conf ]; then
	:
else
	cp $base_path/blog.conf $bin_path 
fi

cp $base_path/go_blog $bin_path

cp -r $base_path/scripts/* $bin_path

cp -r $base_path/templates $bin_path

cp -r $base_path/statics $bin_path

cp $base_path/redis/redis_6666.conf $bin_path/redis

echo "Install OKKKKKK"
echo "run_path $run_path"
