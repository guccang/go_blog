
p=$(dirname $0)

p=$(realpath "$p")

echo $p

# data copy

nohup $p/go_blog $p/blog.conf 2>&1 >> $p/x.log &

sh $p/show.sh
