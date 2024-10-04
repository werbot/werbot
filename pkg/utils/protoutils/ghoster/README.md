Example proto file:

```
syntax = "proto3";

package access;

import "pkg/utils/protoutils/ghoster/proto/ghoster.proto";

message Password {
  string login = 1;
  string password = 2 [(ghoster.secret) = true];
  int32 role = 3 [(ghoster.secret) = true];
}
```

Using in go code:
```
ghoster.Secrets(AccessScheme, false)
```
Secrets(msg proto.Message, nuller bool)  
msg - proto message  
nuller - default false and all string converted in "***", other - removed  

returned if nuller false (removed role):
```
{"password":{"login":"login","password":"***"}}
```

or 

returned if nuller true (removed role)::
```
{"password":{"login":"login"}}
```

