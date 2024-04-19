# 上传数据到百度网盘
p=$(dirname $0)
p=$(realpath $p)
echo $p

prename="blog_datas"

modify_cnt=`find $p -name "*" -mtime -1 | grep -E ".sh|.md|.rdb" |  wc -l`
echo modify_cnt=$modify_cnt
if [ $modify_cnt -le 0 ];then
	exit 0
fi


logpath=~/.bypy/bypy.log

date=`date +%F-%H-%M-%S`
echo $date

msg=`find $p -name "*" -mtime -1 | grep -E ".sh|.md|.rdb"`
echo "$date modify files $msg"
echo "$date modify files $msg" >> "$logpath"

source /etc/profile.d/conda.sh

conda activate py310

bypy info

name="$p/${prename}_${date}.zip"
bypy_remote_path="/go_blogs"



zip $name $p/blogs_txt/* -r
zip -u $name $p/redis/*.rdb -r
bypy upload "$name"  "$bypy_remote_path"

mkdir -p ~/.bypy

echo "$date bypy upload $name to $bypy_remote_path success"
echo "$date bypy upload $name to $bypy_remote_path success" >> $logpath

# clear zip of remove three days ago
rm_cnt=`find $p -name "${prename}_*.zip" -mtime +3 | wc -l`
if [ $rm_cnt -gt 0 ];then
	msg=`find $p -name "${prename}_*.zip" -mtime +3`
	echo "$date bypy remove zip $msg" >> "$logpath"

	find $p -name "${pre_name}_*.zip" -mtime +3 | xargs rm -rf
fi
