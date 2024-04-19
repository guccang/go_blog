
# 上传数据到百度网盘

source /etc/profile.d/conda.sh

conda activate py310

bypy info



date=`date +%F-%H-%M-%S`
echo $date

name="zip_blogs_${date}.zip"
bypy_remote_path="/go_blogs"


blogs_path=./blogs_txt
rdb_path=./redis

zip $name ./blogs_txt/* -r
zip -update $name ./redis/*.rdb
unzip -l $name
bypy upload "$name"  "$bypy_remote_path"
