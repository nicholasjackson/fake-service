#!/bin/sh

# update resolve.conf to add DNSMasq
echo "options ndots:0" > /etc/resolv.conf
echo "nameserver 127.0.0.1" >> /etc/resolv.conf

/usr/sbin/dnsmasq --no-daemon --conf-dir=/etc/dnsmasq.d
