cd cli
GOOS=linux GOARCH=amd64 go build -o mobile-service
chmod +x mobile-service
cp mobile-service /Users/tleyden/Vagrant/couchbase-madhatter
