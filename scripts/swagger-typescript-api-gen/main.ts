import {generateApi} from "swagger-typescript-api";
import {fileURLToPath} from "node:url";
import {dirname, join} from "node:path";

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);
const swaggerDir = join(__dirname, "../../swagger");

await generateApi({
    input: join(swaggerDir, "dash/Dash_swagger.json"),
    output: join(swaggerDir, "dash/typescript"),
    httpClientType: "axios",
    singleHttpClient: true,
    extractRequestBody: true,
    extractResponseBody: true,
    defaultResponseAsSuccess: true,
});
