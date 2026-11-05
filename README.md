# txt2epub

UTF-8 のプレーンテキストを EPUB 3 に変換する、依存関係のない Go 製CLIです。

```sh
go run . -input novel.txt -title "作品名" -author "著者名"
```

出力先を指定する場合:

```sh
go run . -input novel.txt -output output/book.epub -title "作品名" -author "著者名" -lang ja
```

- 空行で段落を区切ります。
- `## 第一章` のように行頭を `## ` にすると、章と目次の項目になります。
- `-output` を省略すると、入力ファイルと同じ場所に `.epub` 拡張子で出力します。

実行ファイルを作る場合は `go build -o txt2epub .` を実行してください。
