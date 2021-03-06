#!/bin/bash

stop_moebot () {
    echo "Removing any old moebot containers..."
    $(docker-compose down)
}

build_and_up () {
    stop_moebot
    echo "Building moebot..."
    $(docker-compose up --build -d)
}

bot () {
    echo "Rebuilding moebot..."
    $(docker-compose up --build -d)
}

db_rebuild () {
    stop_moebot
    echo "Removing old database volume..."
    $(docker volume remove moebot-data)
    echo "Creating new database volume..."
    $(docker volume create moebot-data)
    # Do database rebuild here
}

db_upgrade () {
    echo "db upgrade"
}

follow_logs () {
    $(docker logs -f $1)
}

status () {
    $(docker-compose ps)
}

usage () {
    echo ""
    echo "moebuild.sh usage:"
    echo "Builds and deploys various parts of moebot's system"
    echo "--"
    echo "Parameters:"
    echo ""
    echo "      -k | --stop: Stops all moebot services"
    echo ""
    echo "      -u | --up: Builds and runs the entire moebot ecosystem"
    echo ""
    echo "      -b | --bot: Builds and deploys just moebot's discord bot system"
    echo ""
    echo "      -r | --rebuild: Deletes and rebuilds the moebot database"
    echo ""
    echo "      -d | --upgrade: Performs an upgrade of the database. Does not delete data"
    echo ""
    echo "      -s | --status: Check the current state of moebot"
    echo ""
    echo "      -lm | --logs-moebot: View the logs for moebot's discord bot. Follows automatically"
    echo ""
    echo "      -ld | --logs-db: View the logs for moebot's database. Follows automatically"
}

case "$1" in
    -k|--stop)
        stop_moebot
        ;;
    -u|--up)
        build_and_up
        ;;
    -b|--bot)
        bot
        ;;
    -r|--rebuild)
        db_rebuild
        ;;
    -d|--upgrade)
        db_upgrade
        ;;
    -s|--status)
        status
        ;;
    -lm|--logs-moebot)
        follow_logs "moebot"
        ;;
    -ld|--logs-db)
        follow_logs "db"
        ;;
    *)
        usage
        ;;
 esac
