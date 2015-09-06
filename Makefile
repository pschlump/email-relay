
all:
	go build


# This really only applies to my internal development environment and cross compiling for it.
push_up:
	go build
	git commit -m "Set Build No on files." .
	./updBuildNo.sh email-relay.go
	-rm ~/email-relay.linux  
	tar -czf ~/x.tar *.go ./filelib ./jsonp
	scp ~/x.tar pschlump@192.168.0.182:/home/pschlump
	ssh pschlump@192.168.0.182 "./email-compile.sh"
	scp pschlump@192.168.0.182:/home/pschlump/Projects/email-relay/email-relay ~/email-relay.linux
	cp ~/email-relay.linux . 
	tar -czf ~/x.tar.gz ./content-pusher email-relay.linux 
	( cd ~/aws ; ./to.aws2 ~/x.tar.gz )
	

