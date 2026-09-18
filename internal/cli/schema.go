package cli

const requestSchema = `{
 "$schema":"https://json-schema.org/draft/2020-12/schema",
 "title":"Jev evaluation request",
 "type":"object",
 "additionalProperties":false,
 "required":["state","questions"],
 "properties":{
  "state":{"type":["string","object","array","null"]},
  "model":{"type":"string","default":"jev-latest"},
  "questions":{"type":"object","minProperties":1,"additionalProperties":{"oneOf":[{"$ref":"#/$defs/noul"},{"$ref":"#/$defs/choice"},{"$ref":"#/$defs/score"}]}}
 },
 "$defs":{
  "description":{"type":["string","object","array","null"]},
  "noul":{"type":"object","additionalProperties":false,"required":["type"],"properties":{"type":{"const":"noul"},"instructions":{"$ref":"#/$defs/description"},"criteria":{"type":["object","null"],"additionalProperties":false,"properties":{"true":{"$ref":"#/$defs/description"},"false":{"$ref":"#/$defs/description"}}}}},
  "choice":{"type":"object","additionalProperties":false,"required":["type","criteria"],"properties":{"type":{"const":"choice"},"instructions":{"$ref":"#/$defs/description"},"criteria":{"type":"object","minProperties":1,"additionalProperties":{"$ref":"#/$defs/description"}}}},
  "score":{"type":"object","additionalProperties":false,"required":["type","criteria"],"properties":{"type":{"const":"score"},"instructions":{"$ref":"#/$defs/description"},"criteria":{"type":"array","minItems":2,"items":{"$ref":"#/$defs/description"}}}}
 }
}`
