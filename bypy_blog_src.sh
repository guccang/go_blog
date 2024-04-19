# 上传数据到百度网盘

modify_cnt=`find ./ -name "*" -mtime -1 | wc -l`
echo modify_cnt=$modify_cnt
if [ $modify_cnt -le 0 ];then
	exit 0
fi

logpath=~/.bypy/bypy.log

date=`date +%F-%H-%M-%S`
echo $date

msg=`find ./ -name "*" -mtime -1`
echo "$date modify files $msg" >> "$logpath"

source /etc/profile.d/conda.sh

conda activate py310

bypy info

p=$(dirname $0)
p=$(realpath $p)
echo $p
cd $p


name="$p/blog_src_${date}.zip"
bypy_remote_path="/go_blogs"

find $p/ -name "*.go"  | xargs zip $name 
find $p/ -name "*.mod" | xargs zip -u $name

zip -u $name $p/blog.conf
zip -u $name $p/scripts/*.sh -r
zip -u $name $p/templates/* -r
zip -u $name $p/statics/js/* -r
zip -u $name $p/statics/css/* -r
zip -u $name $p/statics/logo/* -r


unzip -l $name
bypy upload "$name"  "$bypy_remote_path"

mkdir -p ~/.bypy

echo "$date bypy upload $name to $bypy_remote_path success" >> "$logpath"

# clear zip of remove three days ago
rm_cnt=`find $p -name "blog_src_*.zip" -mtime +3 | wc -l`
if [ $rm_cnt -gt 0 ];then
	msg=`find $p -name "blog_src_*.zip" -mtime +3`
	echo "$date bypy remove zip $msg" >> "$logpath"

	find $p -name "blog_src_*.zip" -mtime +3 | xargs rm -rf
fi
