p=$(dirname $0)

p=$(realpath "$p")

base_path=$(dirname $p)
echo "base_path=$base_path"


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

conf_path=$bin_path/blogs_txt

if [ -e $conf_path/sys_conf.md ]; then
	:
else
	cp $base_path/sys_conf.md $conf_path
fi

cp $base_path/go_blog $bin_path

cp -r $base_path/scripts/* $bin_path

cp -r $base_path/templates $bin_path

cp -r $base_path/statics $bin_path

echo "Install OKKKKKK"
echo "run_path $run_path"
