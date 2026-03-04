

p=$(dirname $0)
p=$(realpath "$p")
p=$(dirname "$p")

echo $p

ps aux | grep $p  | grep -Ev "grep|show.sh" 
