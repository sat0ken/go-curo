#!/bin/bash

# rootユーザーが必要
if [ $UID -ne 0 ]; then
  echo "Root privileges are required"
  exit 1;
fi

FLAG=$1

# 全てのnetnsを削除
ip -all netns delete

# 3つのnetnsを作成
ip netns add host1
ip netns add router1
ip netns add host2

# リンクの作成
ip link add name host1-router1 type veth peer name router1-host1 # host1とrouter1のリンク
ip link add name host2-router1 type veth peer name router1-host2 # router1とhost2のリンク

# リンクの割り当て
ip link set host1-router1 netns host1
ip link set router1-host1 netns router1
ip link set host2-router1 netns host2
ip link set router1-host2 netns router1

# host1のリンクの設定
ip netns exec host1 ip addr add 2001:db8:0:1::2/64 dev host1-router1
ip netns exec host1 ip link set host1-router1 up
ip netns exec host1 ethtool -K host1-router1 rx off tx off
ip netns exec host1 ip route add default via 2001:db8:0:1::1

# router1のリンクの設定
ip netns exec router1 ip addr add 2001:db8:0:1::1/64 dev router1-host1
ip netns exec router1 ip link set router1-host1 up
ip netns exec router1 ethtool -K router1-host1 rx off tx off
ip netns exec router1 ip addr add 2001:db8:0:2::1/64 dev router1-host2
ip netns exec router1 ip link set router1-host2 up
ip netns exec router1 ethtool -K router1-router2 rx off tx off
#ip netns exec router1 ip route add 2001:db8::1/64 via 2001:db8::2

# host2のリンクの設定
ip netns exec host2 ip addr add 2001:db8:0:2::2/64 dev host2-router1
ip netns exec host2 ip link set host2-router1 up
ip netns exec host2 ethtool -K host2-router1 rx off tx off
ip netns exec host2 ip route add default via 2001:db8:0:2::1

# icmpv6と転送を無視
# routerを起動する場合はipv6フォワードを無効にする
if [ "$FLAG" == "-router" ]; then
  ip netns exec router1 sysctl -w net.ipv6.icmp.echo_ignore_all=1
  ip netns exec router1 sysctl -w net.ipv6.conf.all.forwarding=0
else
  ip netns exec router1 sysctl -w net.ipv6.conf.all.forwarding=1
fi
