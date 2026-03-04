
p=$(dirname $0)
p=$(realpath $p)

base_path=$(dirname "$p")

if [ -e $base_path/go_blog ];then
	rm $base_path/go_blog
fi

echo $base_path
cd $base_path
go mod tidy

go build 

