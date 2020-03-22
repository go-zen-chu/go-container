#!/usr/bin/env bash

# parse Makefile and show help
awk '/^[a-zA-Z_0-9%:\\\/-]+:/ {
    helpMessage = match(lastLine, /^## (.*)/);
    if (helpMessage) {
        helpCommand = $1;
        helpMessage = substr(lastLine, RSTART + 3, RLENGTH);
     	gsub("\\\\", "", helpCommand);
     	gsub(":+$", "", helpCommand);
        printf "- \033[34m%s\033[0m: %s\n", helpCommand, helpMessage;
    }
}
{ lastLine = $0 }' ./Makefile | sort -u
echo ""
