
p=$(dirname $0)

p=$(realpath "$p")

echo $p

# data copy

nohup $p/go_blog $p/blogs_txt/sys_conf.md 2>&1 >> $p/x.log &

sh $p/show.sh
