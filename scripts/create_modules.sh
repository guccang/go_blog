
modules=("http" "module" "control" "view" "persistence" "mylog" "config" "ioutils" "login" "auth" "comment" "blog" "email" "encryption")
prename="go_blog_pkgs"

cur_path=$(dirname $0)
pkgs_path=$(dirname $cur_path)/go_blog_pkgs
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
