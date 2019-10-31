#!/bin/bash


find . -maxdepth 1 -type f -print0 | xargs -0 -I {} -t -n 1 -P 8 sh -c 'FILE="{}"; echo "$FILE"; until curl -s --fail --show-error -i -X POST \
-F "hash_sum=$(md5sum $FILE | awk '"'{print "'$1'"}'"')" \
-F "file=@$FILE" \
https://receiver.verussensus.com/photos/upload ; do sleep 30 ; done'
