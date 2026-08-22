# Changelog

## 1.0.0 (2026-08-22)


### Features

* add login, callback and logout routes ([3d04b92](https://github.com/choffmann/simple-fileupload/commit/3d04b92a2aeae6db643b8b3339d08952bb11e26c))
* authorize uploads with a session cookie instead of basic auth ([6f058ef](https://github.com/choffmann/simple-fileupload/commit/6f058efcca680aa6680240fb29e55106d65fd141))
* **auth:** support multiple users with bcrypt password hashes ([00e484c](https://github.com/choffmann/simple-fileupload/commit/00e484ce29ca8e4b1aede2797e218105c126d67b))
* **config:** read oidc settings and the session secret ([7c11523](https://github.com/choffmann/simple-fileupload/commit/7c115235826755c7497e279fb7faffc7e7797ab8))
* **config:** read settings from env and add structured logging ([235cc55](https://github.com/choffmann/simple-fileupload/commit/235cc5511e60fa555606d1c3704d5802d4a83caf))
* **config:** require a valid BASE_URL at startup ([d1ee639](https://github.com/choffmann/simple-fileupload/commit/d1ee63929b8d4d6956d219b9e324e8d3503b8d76))
* create docker file ([efac6d5](https://github.com/choffmann/simple-fileupload/commit/efac6d5e91e082fc617d9650bd695b3005480666))
* embed template files ([fd354ab](https://github.com/choffmann/simple-fileupload/commit/fd354ab2823ded8d885c8374597595e16546dec8))
* implement simple file upload ([0fc212a](https://github.com/choffmann/simple-fileupload/commit/0fc212a77f18732185971db5cdbf057c786bbecb))
* **oidc:** derive the area name from preferred_username ([201e6f0](https://github.com/choffmann/simple-fileupload/commit/201e6f07d0d768f4966df852b1c621125bc3313a))
* **oidc:** exchange the auth code and verify the id token ([15525c7](https://github.com/choffmann/simple-fileupload/commit/15525c713981396296433a2ca7c18d0805fd3b52))
* **publicurl:** build escaped public paths and urls ([0e2b984](https://github.com/choffmann/simple-fileupload/commit/0e2b984071f4fce219752aaecc93bb8911c4fb38))
* **qr:** generate qr code png for a url ([06d47d2](https://github.com/choffmann/simple-fileupload/commit/06d47d2c1ea4ff2970b8073751483914bb461b60))
* **qr:** serve a qr page with the public url and png download ([c6016c1](https://github.com/choffmann/simple-fileupload/commit/c6016c1c2dc964a6b42a75beafcdb9f4247527bb))
* redirect to the qr page after an upload ([7a6cba4](https://github.com/choffmann/simple-fileupload/commit/7a6cba4b92c5e03c6bf5c720e9ff9decd8befeaf))
* **session:** sign the logged in user into a cookie ([5f9266c](https://github.com/choffmann/simple-fileupload/commit/5f9266c8133abb961d1e103af56f979d8dad3ad1))
* show the signed in user and a sign out button ([c39d565](https://github.com/choffmann/simple-fileupload/commit/c39d5650df1520f2c07c759edfc24c57de29edec))
* **storage:** add per-user file storage with path traversal guard ([171ad5d](https://github.com/choffmann/simple-fileupload/commit/171ad5d155ec6e15ec7a98155f49daae95c2f87b))
* **storage:** return the stored filename from SaveFile ([ab379d6](https://github.com/choffmann/simple-fileupload/commit/ab379d6f4e44f59c426587018dd6534d8ca3eb0b))
* turn upload form into multi-tenant file browser ([a611cf5](https://github.com/choffmann/simple-fileupload/commit/a611cf5f4e0d1abfbb3f01b49ee8d06de1e288be))


### Bug Fixes

* build browse links from escaped paths and link the qr page ([5eb83a0](https://github.com/choffmann/simple-fileupload/commit/5eb83a016e6ff645e65a7bdc4efbc162ae275faf))
* **docker:** add the ca bundle oidc discovery needs ([f4f045a](https://github.com/choffmann/simple-fileupload/commit/f4f045a08219fd9747c169d4b008b78ff1b67417))
* escape the mkdir redirect path ([980b417](https://github.com/choffmann/simple-fileupload/commit/980b417b8c0c079ce4c345aa76107b8b984237c7))
* handle the errors errcheck flagged ([6a36564](https://github.com/choffmann/simple-fileupload/commit/6a36564855a0c3672ae8fc50b0bc435d71daf2c3))
* **oidc:** reject claims that would share an area instead of folding them ([3ea27e2](https://github.com/choffmann/simple-fileupload/commit/3ea27e21df6d989ceadb243e2891628cccc53abd))
* **oidc:** restore the specified area name conversion ([8b09263](https://github.com/choffmann/simple-fileupload/commit/8b09263e3de50e934f7d5528c8a874998f912896))
* reject cross origin posts to the state changing routes ([0928529](https://github.com/choffmann/simple-fileupload/commit/0928529973444d1b829c75b779ed4a9e8518bf9c))
* stop uploaded files from running as first party script ([5360fbd](https://github.com/choffmann/simple-fileupload/commit/5360fbd6fcccc74cd2129806db569c4b52e411b8))
* **storage:** reject usernames that escape the upload directory ([db7b238](https://github.com/choffmann/simple-fileupload/commit/db7b23852d9878e3fb864242f827bf5aeed06f17))
