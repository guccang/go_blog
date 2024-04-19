
modules=("http" "module" "control" "view" "http_template" "persistence" "protocol" "mylog" "config" "ioutils" "login" "auth")
prename="go_blog_pkgs"

cd ..

base_path=$(pwd)/$prename
mkdir -p $base_path
for m in ${modules[@]};do
	cd $base_path
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
