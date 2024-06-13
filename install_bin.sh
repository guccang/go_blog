

cp ./*.pem ./bin

cd ./bin

sh ./stop.sh

cd ..

sh ./scripts/install.sh bin

cd ./bin

sh ./start.sh
