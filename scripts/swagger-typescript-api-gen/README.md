# typescript-api

Generates a TypeScript client from `swagger/dash/Dash_swagger.json` into
`swagger/dash/typescript`. Run `npm run gen` (or `make gen/dts`) after
`make gen/docs`.

The generated client is a plain `axios`-based API class and can be consumed
directly by any dashboard frontend.
