

date=`date +%F-%H-%M-%S`
echo $date

name="zip_blogs_${date}"

blogs_path=./blogs_txt
rdb_path=./redis


zip $name ./blogs_txt/* -r

zip -update $name ./redis/*.rdb

unzip -l $name
