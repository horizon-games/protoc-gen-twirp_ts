# Typescript proto plugin for Twirp


## Installation

```
go get -u github.com/horizon-games/protoc-gen-twirp_ts
```

## Usage

Use the `protoc` tool to invoke the plugin and generate typescript files:

```
protoc --proto_path=./proto/ --twirp_ts_out=./out/ service.proto
```

Example usage:

```js
const fetch = require('node-fetch');

const api = require('./api');

const svc = new api.Service('https://grpc.example.com', fetch);

svc.ping().then((pong) => {
  console.log({pong})
}).catch((err) => {
  console.log({err})
});
```

> The MIT License
>
> Copyright 2018 Horizon Blockchain Games, Inc.
>
> Permission is hereby granted, free of charge, to any person obtaining a copy of
> this software and associated documentation files (the "Software"), to deal in
> the Software without restriction, including without limitation the rights to
> use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies
> of the Software, and to permit persons to whom the Software is furnished to do
> so, subject to the following conditions:
>
> The above copyright notice and this permission notice shall be included in all
> copies or substantial portions of the Software.
>
> THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
> IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
> FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
> AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
> LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
> OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
> SOFTWARE.
