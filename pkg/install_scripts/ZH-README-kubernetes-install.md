# 以kubernetes方式部署dory

## 概要

1. 请根据 `README-kubernetes-install.md` 的说明手工安装dory
2. 请根据 `README-kubernetes-config.md` 的说明在完成安装后手工设置dory
3. 假如安装失败，请根据 `README-kubernetes-reset.md` 的说明停止所有dory服务并重新安装

## 创建相关根目录

```shell script
{{- if $.imageRepoInternal }}
# 创建 {{ $.imageRepo.type }} 相关目录并设置目录权限
mkdir -p {{ $.rootDir }}/{{ $.imageRepo.internal.namespace }}/database
mkdir -p {{ $.rootDir }}/{{ $.imageRepo.internal.namespace }}/jobservice
mkdir -p {{ $.rootDir }}/{{ $.imageRepo.internal.namespace }}/redis
mkdir -p {{ $.rootDir }}/{{ $.imageRepo.internal.namespace }}/registry
chown -R 999:999 {{ $.rootDir }}/{{ $.imageRepo.internal.namespace }}/database
chown -R 10000:10000 {{ $.rootDir }}/{{ $.imageRepo.internal.namespace }}/jobservice
chown -R 999:999 {{ $.rootDir }}/{{ $.imageRepo.internal.namespace }}/redis
chown -R 10000:10000 {{ $.rootDir }}/{{ $.imageRepo.internal.namespace }}/registry
ls -alh {{ $.rootDir }}/{{ $.imageRepo.internal.namespace }}
{{- end }}

# 创建 dory 组件相关目录并设置目录权限
mkdir -p {{ $.rootDir }}/{{ $.dory.namespace }}/dory-core/dory-data
mkdir -p {{ $.rootDir }}/{{ $.dory.namespace }}/dory-core/tmp
cp -rp {{ $.dory.namespace }}/{{ $.dory.docker.dockerName }} {{ $.rootDir }}/{{ $.dory.namespace }}/
cp -rp {{ $.dory.namespace }}/dory-core {{ $.rootDir }}/{{ $.dory.namespace }}/
chown -R 1000:1000 {{ $.rootDir }}/{{ $.dory.namespace }}/dory-core
mkdir -p {{ $.rootDir }}/{{ $.dory.namespace }}/mongo-core-dory
chown -R 999:999 {{ $.rootDir }}/{{ $.dory.namespace }}/mongo-core-dory
ls -alh {{ $.rootDir }}/{{ $.dory.namespace }}
```

## {{ $.imageRepo.type }} 安装配置

```shell script
{{- if $.imageRepoInternal }}
# 创建 {{ $.imageRepo.type }} 名字空间与pv
kubectl delete ns {{ $.imageRepo.internal.namespace }}
kubectl delete pv {{ $.imageRepo.internal.namespace }}-pv
kubectl apply -f {{ $.imageRepo.internal.namespace }}/step01-namespace-pv.yaml

# 使用helm安装 {{ $.imageRepo.type }}
helm install -n {{ $.imageRepo.internal.namespace }} {{ $.imageRepo.internal.namespace }} {{ $.imageRepo.type }}
helm -n {{ $.imageRepo.internal.namespace }} list

# 等待所有 {{ $.imageRepo.type }} 服务状态为 ready
kubectl -n {{ $.imageRepo.internal.namespace }} get pods -o wide

# 创建 {{ $.imageRepo.type }} 自签名证书并复制到 /etc/docker/certs.d
sh {{ $.imageRepo.internal.namespace }}/harbor_update_docker_certs.sh
ls -alh /etc/docker/certs.d/{{ $.imageRepoDomainName }}
{{- else }}
# 把{{ $.imageRepo.type }}服务器({{ $.imageRepoIp }})上的证书复制到本节点的 /etc/docker/certs.d/{{ $.imageRepoDomainName }} 目录
# 证书文件包括: ca.crt, {{ $.imageRepoDomainName }}.cert, {{ $.imageRepoDomainName }}.key
{{- end }}

# 在当前主机以及所有kubernetes节点主机上，把 {{ $.imageRepo.type }} 的域名记录添加到 /etc/hosts
vi /etc/hosts
{{ $.imageRepoIp }}  {{ $.imageRepoDomainName }}

# 设置docker客户端登录到 {{ $.imageRepo.type }}
docker login --username {{ $.imageRepoUsername }} --password {{ $.imageRepoPassword }} {{ $.imageRepoDomainName }}

# 在 {{ $.imageRepo.type }} 中创建 public, hub, gcr, quay 四个项目
curl -k -X POST -H 'Content-Type: application/json' -d '{"project_name": "public", "public": true}' 'https://{{ $.imageRepoUsername }}:{{ $.imageRepoPassword }}@{{ $.imageRepoDomainName }}/api/v2.0/projects'
curl -k -X POST -H 'Content-Type: application/json' -d '{"project_name": "hub", "public": true}' 'https://{{ $.imageRepoUsername }}:{{ $.imageRepoPassword }}@{{ $.imageRepoDomainName }}/api/v2.0/projects'
curl -k -X POST -H 'Content-Type: application/json' -d '{"project_name": "gcr", "public": true}' 'https://{{ $.imageRepoUsername }}:{{ $.imageRepoPassword }}@{{ $.imageRepoDomainName }}/api/v2.0/projects'
curl -k -X POST -H 'Content-Type: application/json' -d '{"project_name": "quay", "public": true}' 'https://{{ $.imageRepoUsername }}:{{ $.imageRepoPassword }}@{{ $.imageRepoDomainName }}/api/v2.0/projects'

# 把之前拉取的docker镜像推送到 {{ $.imageRepo.type }}
{{- range $_, $image := $.dockerImages }}
docker tag {{ if $image.dockerFile }}{{ $image.target }}{{ else }}{{ $image.source }}{{ end }} {{ $.imageRepoDomainName }}/{{ $image.target }}
{{- end }}

{{- range $_, $image := $.dockerImages }}
docker push {{ $.imageRepoDomainName }}/{{ $image.target }}
{{- end }}
```

## 把dory组件部署到kubernetes中

```shell script
# 创建 {{ $.dory.namespace }} 组件的名字空间与pv
kubectl delete ns {{ $.dory.namespace }}
kubectl delete pv {{ $.dory.namespace }}-pv
kubectl apply -f {{ $.dory.namespace }}/step01-namespace-pv.yaml

# 创建 docker executor 自签名证书
sh {{ $.dory.namespace }}/{{ $.dory.docker.dockerName }}/docker_certs.sh
kubectl -n {{ $.dory.namespace }} create secret generic {{ $.dory.docker.dockerName }}-tls --from-file=certs/ca.crt --from-file=certs/tls.crt --from-file=certs/tls.key --dry-run=client -o yaml | kubectl apply -f -
kubectl -n {{ $.dory.namespace }} describe secret {{ $.dory.docker.dockerName }}-tls
rm -rf certs

# 复制 {{ $.imageRepo.type }} 证书到docker配置目录
cp -rp /etc/docker/certs.d/{{ $.imageRepoDomainName }} {{ $.rootDir }}/{{ $.dory.namespace }}/{{ $.dory.docker.dockerName }}

{{- if $.artifactRepoInternal }}
# 从docker镜像中复制nexus初始化数据
docker rm -f nexus-data-init || true
docker run -d -t --name nexus-data-init doryengine/nexus-data-init:alpine-3.15.3 cat
docker cp nexus-data-init:/nexus-data/nexus {{ $.rootDir }}/{{ $.dory.namespace }}
docker rm -f nexus-data-init
chown -R 200:200 {{ $.rootDir }}/{{ $.dory.namespace }}/nexus
ls -alh {{ $.rootDir }}/{{ $.dory.namespace }}/nexus
{{- end }}

# 在kubernetes中部署dory组件
kubectl apply -f {{ $.dory.namespace }}/step02-statefulset.yaml
kubectl apply -f {{ $.dory.namespace }}/step03-service.yaml

# 检查dory服务状态
kubectl -n {{ $.dory.namespace }} get pods -o wide
```

## 在kubernetes中创建project-data-alpine pod

```shell script
# project-data-alpine pod 用于创建项目的应用文件目录
# 在kubernetes中创建project-data-alpine pod
kubectl apply -f project-data-alpine.yaml
kubectl -n {{ $.dory.namespace }} get pods
```

## 请继续完成dory的配置

2. 请根据 `README-kubernetes-config.md` 的说明在完成安装后手工设置dory
