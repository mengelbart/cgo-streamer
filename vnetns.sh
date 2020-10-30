#!/bin/bash

#set -e
#set -o pipefail

check_sudo() {
	if [ "$EUID" -ne 0 ]; then
		echo "Please run as root."
		exit
	fi
}

up() {
	echo "setup network"
	ip netns add ns1
	ip netns add ns2

	ip link add veth1 type veth peer name br-veth1
	ip link add veth2 type veth peer name br-veth2

	ip link set veth1 netns ns1
	ip link set veth2 netns ns2

	ip netns exec ns1 ip addr add 192.168.1.11/24 dev veth1
	ip netns exec ns2 ip addr add 192.168.1.12/24 dev veth2

	ip link add name br1 type bridge
	ip link set br1 up

	ip link set br-veth1 up
	ip link set br-veth2 up

	ip netns exec ns1 ip link set veth1 up
	ip netns exec ns2 ip link set veth2 up

	ip link set br-veth1 master br1
	ip link set br-veth2 master br1
}

down() {
	echo "delete network"
	ip netns del ns1
	ip netns del ns2

	ip link del br1
}


main() {
	check_sudo
	local cmd=$1
	if [[ $cmd == "up" ]]; then
		up
	elif [[ $cmd == "down" ]]; then
		down
	fi
}

main "$@"

