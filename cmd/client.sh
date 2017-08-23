# client post [filename]
# client put [URL] [filename]
# client delete [URL]

URL="http://mydevelop/articles/"
ProfileURL="http://mydevelop/profile"

case "$1" in
"post")
	curl -X POST -H "Content-Type: text/plain" $URL --data-binary @"$2"
	;;
"delete")
	curl -X DELETE "$URL""$2"
	;;
"put")
	curl -X PUT -H "Content-Type: text/plain" "$URL""$2" --data-binary @"$3"
	;;
"profile")
	curl -X PUT -H "Content-Type: text/plain" "$ProfileURL" --data-binary @"$2"
	;;
esac

