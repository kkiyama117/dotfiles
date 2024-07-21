LAST_REPO=""
cd() { 
    z "$@";
    git rev-parse 2>/dev/null;

    if [ $? -eq 0 ]; then
        if [ "$LAST_REPO" != $(basename $(git rev-parse --show-toplevel)) ]; then
        onefetch
        LAST_REPO=$(basename $(git rev-parse --show-toplevel))
        fi
    fi
}
