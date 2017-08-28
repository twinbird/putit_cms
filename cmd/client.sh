# client post [filename]
# client put [URL] [filename]
# client delete [URL]

URL="http://mydevelop/articles/"
ProfileURL="http://mydevelop/profile"
FileURL="http://mydevelop/static/"
authUser="user"
authPassword="password"
hashingCommand="echo -n "$authPassword" | shasum -a 256 | tr -d ' *-'"
sha256Password=$(eval $hashingCommand)

case "$1" in
"post")
	curl -u $authUser":"$sha256Password -X POST -H "Content-Type: text/plain" $URL --data-binary @"$2"
	;;
"delete")
	curl -u $authUser":"$sha256Password -X DELETE "$URL""$2"
	;;
"put")
	curl -u $authUser":"$sha256Password -X PUT -H "Content-Type: text/plain" "$URL""$2" --data-binary @"$3"
	;;
"profile")
	curl -u $authUser":"$sha256Password -X PUT -H "Content-Type: text/plain" "$ProfileURL" --data-binary @"$2"
	;;
"file")
	curl -u $authUser":"$sha256Password -X PUT -H "Content-Type: text/plain" "$FileURL""$2" --data-binary @"$2"
	;;
esac

