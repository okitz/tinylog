# tinylog

## プロジェクト概要

**tinylog** は、Go/TinyGoで実装されたMQTTベースの分散ログシステムです。  
MQTTクライアントとして機能し、以下を並列に実行することでの自律的にデータを蓄積します。
- Publisherとしてデータをクラスタネットワークに送信
- Subscriber/Raftノードとしてデータログを安定的に保持
最終的にIoTデバイス(マイコン)への移植を想定し、TinyGoでビルド可能にすることを目指しています。

- Wi-Fi接続に対応したマイコン(Raspberry Pi Pico W等)での動作を想定し、TinyGoでビルド可能(WIP)


## 使い方

### TinyGo でのビルド手順

Raspberry Pi Pico W での動作を目的としています（※開発中）。
1. **Protocol Buffers のコード生成**  
以下のコマンドで `.proto` ファイルからAPIの型定義を生成
```bash
make protoc
```
2. Wi-Fi情報の設定
`internal/pico/` ディレクトリ内に以下のファイルを作成
- `ssid.text`：接続するWi-FiのSSID
- `password.text`：対応するパスワード
詳細は依存リポジトリ [soypat/cyw43439](https://github.com/soypat/cyw43439) のREADMEを参照

3. TinyGoバイナリのビルド
MQTTブローカのIPアドレスを`BROKER_IP`で指定してください。
```bash
BROKER_IP=x.x.x.x make tinybuild
```
> ⚠️ マイコン実機上での動作は現在開発中です。

### PC 上での開発環境構築
1. **Devcontainerを使う場合**  
VSCodeで `.devcontainer/devcontainer.json` を参照してコンテナを起動
- 作業用の `tinylog-app`コンテナと MQTTブローカ `tinylog-mosquitto`コンテナが同時に起動します
- ポート 1883 から Moquittoに接続

2. **ローカルビルド**
MQTTブローカのIPアドレスを`BROKER_IP`で指定してください。
```bash
# amd64用バイナリをビルド
BROKER_IP=x.x.x.x make build
```

## ライセンス
This project includes code derived from github.com/soypat/cyw43439, licensed under the MIT License.
See LICENSES/cyw43439.MIT for details.