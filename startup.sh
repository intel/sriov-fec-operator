virsh destroy lb
virsh destroy master1
virsh destroy master2
virsh destroy master3
virsh destroy worker1
virsh destroy worker2

virsh net-start --network default

virsh start lb
virsh start master1
virsh start master2
virsh start master3
virsh start worker1
virsh start worker2

WEB_PORT=8080
install_dir=/disks/
docker run -d --name static-file-server --rm  -v ${install_dir}:/web -p ${WEB_PORT}:${WEB_PORT} -u $(id -u):$(id -g) halverneus/static-file-server:latest