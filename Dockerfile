FROM golang:1.6.0
#FROM index.alauda.cn/library/golang:1.8.5

ENV BROKERPORT 8888
EXPOSE 8888

ENV TIME_ZONE=Asia/Shanghai
RUN ln -snf /usr/share/zoneinfo/$TIME_ZONE /etc/localtime && echo $TIME_ZONE > /etc/timezone

#ENV GOPATH=/xxxxx/
COPY . /go/src/github.com/asiainfoLDP/datafoundry_servicebroker_openshift

WORKDIR /go/src/github.com/asiainfoLDP/datafoundry_servicebroker_openshift

#RUN go get github.com/tools/godep \
#    && godep go build 

RUN go build -ldflags="-X github.com/asiainfoLDP/datafoundry_servicebroker_openshift/handler.Gitrev=`./git-version.sh`"

CMD ["sh", "-c", "./datafoundry_servicebroker_openshift"]
