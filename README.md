# tinylog

## プロジェクト概要

**tinylog** は、Go/TinyGoで実装されたMQTTベースの分散ログシステムです。  
MQTTクライアントとして機能し、以下を並列に実行することでの自律的にデータを蓄積します。
- Publisherとしてデータをクラスタネットワークに送信
- Subscriber/Raftノードとしてデータログを安定的に保持
最終的にIoTデバイス(マイコン)への移植を想定し、TinyGoでビルド可能にすることを目指しています。

- Wi-Fi接続に対応したマイコン(Raspberry Pi Pico W等)での動作を想定し、TinyGoでビルド可能(WIP)


## 使い方

### 開発環境構築
VSCodeで `.devcontainer/devcontainer.json` を参照してコンテナを起動してください。
- 作業用の `tinylog-app`コンテナと MQTTブローカ `tinylog-mosquitto`コンテナが同時に起動します
- Moquittoは`1883`番ポートで待ち受け

### Go でのビルド(AMD64 PC向け)
1. **Protocol Buffers のコード生成** (共通) 
以下のコマンドで `.proto` ファイルからAPIの型定義を生成
```bash
make protoc
```
2. **Goバイナリのビルド**
`cmd/main_amd64.go`をエントリポイントとして `build/amd64/app` がビルドされます。
```bash
# amd64用バイナリをビルド
make build
```
`tinylog-mosquitto` 以外のMQTTブローカを使う場合、IPアドレスを`BROKER_IP`で指定してください。


> 複数ノードでの動作確認にはテストコードを走らせてログを見るのがわかりやすいです。
> ```bash
> go test -v ./test -run TestMQTTElectionBasic
> ```

### TinyGo でのビルド (RPI PICO W向け)
Raspberry Pi Pico W での動作を前提としています（※開発中）。
1. **Protocol Buffers のコード生成** (共通) 
以下のコマンドで `.proto` ファイルからAPIの型定義を生成
```bash
make protoc
```
2. **Wi-Fi情報の設定**  
`internal/pico/` ディレクトリ内に以下のファイルを作成
- `ssid.text`：接続するWi-FiのSSID
- `password.text`：対応するパスワード
詳細は依存リポジトリ [soypat/cyw43439](https://github.com/soypat/cyw43439) のREADMEを参照

3. **TinyGoバイナリのビルド**
`cmd/main_pico.go`をエントリポイントとして `build/tinygo/app.uf2` がビルドされます。
MQTTブローカのIPアドレスを `BROKER_IP` 環境変数で指定してください。
```bash
BROKER_IP=x.x.x.x make tinybuild
```
> ⚠️ マイコン実機上での動作は現在開発中です。


## ライセンス
This project includes code derived from github.com/soypat/cyw43439, licensed under the MIT License.
See LICENSES/cyw43439.MIT for details.