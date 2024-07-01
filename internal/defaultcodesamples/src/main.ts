import commandLineArgs from "command-line-args";
import {
  SupportedTargets,
  getSupportedLanguages,
} from "@readme/oas-to-snippet/languages";
import {
  readFilePathOrUrl,
  generateCodeSamplesOverlay,
} from "./generateCodeSamplesOverlay";

async function main() {
  const options = commandLineArgs([
    { name: "schema", alias: "s", type: String },
    { name: "language", alias: "l", type: String },
  ]);
  const filePathOrUrl = options.schema;
  const language = options.language;

  if (!filePathOrUrl) {
    throw new Error("Please provide a schema file");
  }

  if (!language || typeof language !== "string") {
    throw new Error("Please provide a language");
  }

  const supportedLanguages = getSupportedLanguages();

  const isSupportedLanguage = (x: string): x is SupportedTargets =>
    x in supportedLanguages;

  if (!isSupportedLanguage(language)) {
    throw new Error(
      `Language not supported: ${language}. Supported languages: ${Object.keys(
        supportedLanguages
      ).join(", ")}`
    );
  }

  // Read the file
  const fileContent = await readFilePathOrUrl(filePathOrUrl);

  const { overlay, errors } = await generateCodeSamplesOverlay(
    fileContent,
    language
  );

  process.stdout.write(JSON.stringify(overlay, null, 2));
  for (const error of errors || []) {
    process.stderr.write(
      `Error generating code sample for ${error.jsonPathSelector}: ${error.error}`
    );
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
