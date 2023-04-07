# go-curo

2022年インターフェイス11月号のソフトウェアルータをgoに移植してみました。

## 元ネタ

https://github.com/kametan0730/interface_2022_11  
https://github.com/kametan0730/curo

## 実行方法

1. 以下のコマンドでルータをビルドします

```shell
$ go build .
```

2. network namespaceを作るスクリプトを実行します

```shell
$ sudo ./netns-scripts/chapter4-2-netns.sh
```

3．ルータを起動します

※章ごとに引数で起動させてルータの挙動を変えます

```shell
$ sudo ip netns exec router1 ./main -mode ch1 # 1章の内容
$ sudo ip netns exec router1 ./main -mode ch2 # 2~4章の内容
$ sudo ip netns exec router1 ./main -mode ch5 # 5章の内容
```