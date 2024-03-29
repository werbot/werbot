package license

import (
  "context"
  "encoding/json"
  "os"
  "time"

  "github.com/google/uuid"
  "google.golang.org/protobuf/types/known/timestamppb"

  "github.com/werbot/werbot/internal"
  licensepb "github.com/werbot/werbot/internal/grpc/license/proto"
  license_lib "github.com/werbot/werbot/internal/license"
  "github.com/werbot/werbot/internal/trace"
)

// License is ...
func (h *Handler) License(ctx context.Context, in *licensepb.License_Request) (*licensepb.License_Response, error) {
  licenseOpen := true
  licensePublic := internal.GetString("LICENSE_KEY_PUBLIC", "")
  response := &licensepb.License_Response{}

  readFile, err := os.ReadFile(internal.GetString("LICENSE_FILE", "/license.key"))
  if err != nil {
    licenseOpen = false
    return nil, trace.Error(err, log, trace.MsgFailedToOpenLicenseFile)
  }

  if licensePublic == "" {
    licenseOpen = false
  }

  if licenseOpen {
    lic, err := license_lib.DecodePublicKey([]byte(licensePublic))
    if err != nil {
      return nil, trace.Error(err, log, trace.MsgLicenseKeyIsBroken)
    }

    // The main information of the license
    licDecode, err := lic.Decode(readFile)
    if err != nil {
      return nil, trace.Error(err, log, trace.MsgLicenseStructureIsBroken)
    }
    response.Issued = licDecode.License.Iss
    response.Customer = licDecode.License.Cus
    response.Subscriber = licDecode.License.Sub
    response.Type = licDecode.License.Typ
    response.IssuedAt = timestamppb.New(licDecode.License.Iat)
    response.ExpiresAt = timestamppb.New(licDecode.License.Exp)
    response.Expired = lic.Expired()

    licData := map[string]any{}
    if err := json.Unmarshal(licDecode.License.Dat, &licData); err != nil {
      return nil, trace.Error(err, log, trace.MsgStructureIsBroken)
    }

    response.Limits = map[string]int32{
      "Companies": int32(licData["companies"].(float64)),
      "Servers":   int32(licData["servers"].(float64)),
      "Users":     int32(licData["users"].(float64)),
    }

    for _, item := range licData["modules"].([]interface{}) {
      response.Modules = append(response.Modules, item.(string))
    }
  } else {
    now := time.Now()

    response.Issued = "free"
    response.Customer = "Mr. Robot"
    response.Subscriber = uuid.New().String()
    response.Type = "open source"
    response.IssuedAt = timestamppb.New(now.UTC())
    response.ExpiresAt = timestamppb.New(now.AddDate(0, 0, 365).UTC())
    response.Expired = true

    response.Modules = []string{"module1", "module2", "module3"}
    response.Limits = map[string]int32{
      "Companies": 99,
      "Servers":   99,
      "Users":     99,
    }
  }

  return response, nil
}
