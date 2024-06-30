import YAML, { JSON_SCHEMA } from 'js-yaml';
/**
 * Determine if a given variable is a `Buffer`.
 *
 */
export function isBuffer(obj: any) {
  return (
    obj != null &&
    obj.constructor != null &&
    typeof obj.constructor.isBuffer === 'function' &&
    obj.constructor.isBuffer(obj)
  );
}

/**
 * Converts GitHub blob URLs to raw URLs
 */
export function normalizeURL(url: string) {
  if (url.startsWith('https://github.com/') && url.includes('/blob/')) {
    return url.replace('github.com', 'raw.githubusercontent.com').replace('/blob/', '/');
  }
  return url;
}

/**
 * Determine the type of a given variable. Returns `false` if unrecognized.
 *
 */
export function getType(obj: any) {
  if (isBuffer(obj)) {
    return 'buffer';
  } else if (typeof obj === 'object') {
    return 'json';
  } else if (typeof obj === 'string') {
    if (obj.match(/\s*{/)) {
      return 'string-json';
    } else if (obj.match(/\n/)) {
      // Not sure about this...
      return 'string-yaml';
    } else if (obj.substring(0, 4) === 'http') {
      return 'url';
    }

    return 'path';
  }

  return false;
}

/**
 * Determine if a given schema if an OpenAPI definition.
 *
 */
export function isOpenAPI(schema: Record<string, unknown>) {
  return !!schema.openapi;
}

/**
 * Determine if a given schema is a Postman collection.
 *
 * Unfortunately the Postman schema spec doesn't have anything like `openapi` or `swagger` for us
 * to look at but it does require that `info` and `item` be present and as `item` doesn't exist in
 * OpenAPI or Swagger we can use the combination of those two properties to determine if what we
 * have is a Postman collection.
 *
 * @see {@link https://schema.postman.com/json/collection/v2.0.0/collection.json}
 * @see {@link https://schema.postman.com/json/collection/v2.1.0/collection.json}
 */
export function isPostman(schema: Record<string, unknown>): boolean {
  return !!schema.info && !!schema.item;
}

/**
 * Determine if a given schema if an Swagger definition.
 *
 */
export function isSwagger(schema: Record<string, unknown>) {
  return !!schema.swagger;
}

/**
 * Convert a YAML blob or stringified JSON object into a JSON object.
 *
 */
export function stringToJSON(string: string | Record<string, unknown>) {
  if (typeof string === 'object') {
    return string;
  } else if (string.match(/^\s*{/)) {
    return JSON.parse(string);
  }

  return YAML.load(string, { schema: JSON_SCHEMA });
}

/**
 * Determine if a given schema is an API definition that we can support.
 *
 */
export function isAPIDefinition(schema: Record<string, unknown>) {
  return isOpenAPI(schema) || isPostman(schema) || isSwagger(schema);
}

/**
 * Retrieve the type of API definition that a given schema is.
 *
 */
export function getAPIDefinitionType(schema: Record<string, unknown>) {
  if (isOpenAPI(schema)) {
    return 'openapi';
  } else if (isPostman(schema)) {
    return 'postman';
  } else if (isSwagger(schema)) {
    return 'swagger';
  }

  return 'unknown';
}
