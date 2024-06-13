
modules=("http" "module" "control" "view" "persistence" "mylog" "config" "ioutils" "login" "auth" "comment" "blog" "email" "encryption" "search" "share")
prename="pkgs"

cur_path=$(realpath $0)
echo $cur_path
cur_path=$(dirname $cur_path)
echo $cur_path
cur_path=$(dirname $cur_path)
echo $cur_path
pkgs_path=$(dirname $cur_path)/pkgs
pkgs_path=$(realpath $pkgs_path)

mkdir -p $pkgs_path
for m in ${modules[@]};do
	cd $pkgs_path
	p=$(realpath $m)
	mkdir -p $p
	echo "$m OK"
	if [ -e $p/go.mod ];then
		:
	else
		cd $p
		echo "go mod tidy" >> $p/build.sh
		echo "go build" >> $p/build.sh
		touch $m.go
		echo "package $m" >> $m.go
		echo "import (" >> $m.go
		echo ")" >> $m.go
		go mod init $m
	fi
done
