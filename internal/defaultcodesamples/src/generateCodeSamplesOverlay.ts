import fs from "fs";
import Oas from "oas";
import oasToSnippet from "@readme/oas-to-snippet";
import { SupportedTargets } from "@readme/oas-to-snippet/languages";
import { DataForHAR } from "oas/types";
import { AuthForHAR } from "@readme/oas-to-har/lib/types";
import { Operation } from "oas/operation";
import OASNormalize from "./oas-normalize";

type CodeSample = {
  lang: string;
  label?: string;
  source: string;
  jsonPathSelector: string;
};

type CodeSampleError = {
  jsonPathSelector: string;
  error: string;
};

export async function generateCodeSamplesOverlay(
  fileContent: string,
  language: SupportedTargets
) {
  const codeSamples: CodeSample[] = [];
  const errors: CodeSampleError[] = [];

  // We need to normalize for two reasons:
  // * The OAS class expects a JSON string
  // * Dereferencing the OAS object will improve examples
  const oas = new OASNormalize(fileContent);
  const doc = await oas.deref();
  const docAsJsonString = JSON.stringify(doc, null, 2);
  const apiDefinition = new Oas(docAsJsonString);
  for (const [pathName, path] of Object.entries(apiDefinition.getPaths())) {
    for (const [method, operation] of Object.entries(path)) {
      const res = generateCodeSampleForOperation(
        operation,
        pathName,
        method,
        apiDefinition,
        language
      );
      if ("error" in res) {
        errors.push(res);
      } else {
        codeSamples.push(res);
      }
    }
  }

  const overlay = buildCodeSamplesOverlay(codeSamples);
  return { overlay, errors: errors.length > 0 ? errors : undefined };
}

type CodeSampleResult = CodeSampleError | CodeSample;

function generateCodeSampleForOperation(
  operation: Operation,
  pathName: string,
  method: string,
  apiDefinition: Oas,
  language: SupportedTargets
): CodeSampleResult {
  const exampleGroups = Object.values(operation.getExampleGroups());
  const jsonPathSelector = buildJsonPathSelector(pathName, method);

  const buildError = (error: string) => ({
    jsonPathSelector,
    error,
  });

  if (exampleGroups.length > 0) {
    return buildError(`Multiple example groups are not supported`);
  } else {
    const requestBodyExamples = operation.getRequestBodyExamples();
    if (requestBodyExamples?.length > 1) {
      return buildError(`Multiple requestBodyExamples are not supported`);
    }

    if (requestBodyExamples[0]?.examples?.length > 1) {
      return buildError(
        `Multiple requestBodyExamples[0].examples are not supported`
      );
    }

    const firstBodyExample = requestBodyExamples?.[0]?.examples[0];

    const dataForHAR: DataForHAR = {
      body: firstBodyExample?.value,
    };

    for (const param of operation.getParameters()) {
      if (param.example) {
        dataForHAR[param.in] =
          dataForHAR[param.in] || ({} as Record<string, any>);
        dataForHAR[param.in]![param.name] = param.example;
      }
      const examples = Object.values(param.examples || {});
      if (examples?.length > 0) {
        return buildError(`Multiple parameter examples are not supported`);
      }
    }

    const auth: AuthForHAR = {};
    for (const securityRequirement of operation.getSecurityWithTypes()) {
      for (const sec of securityRequirement || []) {
        if (!sec) continue;
        auth[sec.security._key] =
          `MY_` + sec.security._key.replace(/[^a-zA-Z0-9]/g, "_").toUpperCase();
      }
    }

    const { code, highlightMode } = oasToSnippet(
      apiDefinition,
      operation,
      dataForHAR,
      auth,
      language
    );
    if (code && highlightMode) {
      return {
        lang: highlightMode,
        source: code,
        jsonPathSelector: buildJsonPathSelector(pathName, method),
      };
    }

    return buildError(`Could not generate code snippet`);
  }
}

export function readFilePathOrUrl(filePathOrUrl: string) {
  if (
    filePathOrUrl.startsWith("https://") ||
    filePathOrUrl.startsWith("http://")
  ) {
    return fetch(filePathOrUrl).then((res) => res.text());
  }

  if (!fs.existsSync(filePathOrUrl)) {
    throw new Error(`File does not exist: ${filePathOrUrl}`);
  }

  return fs.readFileSync(filePathOrUrl, "utf8");
}

function buildCodeSamplesOverlay(codeSamples: CodeSample[]) {
  const overlay = {
    overlay: "1.0.0",
    info: {
      title: "Code Samples",
      version: "0.0.0",
    },
    actions: codeSamples.map((codeSample) => {
      return {
        target: codeSample.jsonPathSelector,
        update: {
          "x-codeSamples": [
            {
              lang: codeSample.lang,
              label: codeSample.label,
              source: codeSample.source,
            },
          ],
        },
      };
    }),
  };

  return overlay;
}
function buildJsonPathSelector(pathName: string, method: string) {
  return `$["paths"]["${pathName}"]["${method}"]`;
}
