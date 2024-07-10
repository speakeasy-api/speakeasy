import type { Options } from './lib/types';
import type { OpenAPI } from 'openapi-types';

import fs from 'node:fs';

import openapiParser from '@readme/openapi-parser';
import postmanToOpenAPI from '@readme/postman-to-openapi';

import converter from 'swagger2openapi';

import * as utils from './lib/utils';

export default class OASNormalize {
  cache: {
    bundle?: false | OpenAPI.Document;
    deref?: false | OpenAPI.Document;
    load?: false | Record<string, unknown>;
  };

  file: any;

  opts: Options;

  type: ReturnType<typeof utils.getType>;

  constructor(file: any, opts?: Options) {
    this.file = file;
    this.opts = {
      colorizeErrors: false,
      enablePaths: false,
      ...opts,
    };

    this.type = utils.getType(this.file);

    this.cache = {
      load: false,
      bundle: false,
      deref: false,
    };
  }

  /**
   * @private
   */
  async load() {
    if (this.cache.load) return Promise.resolve(this.cache.load);

    const resolve = (obj: Parameters<typeof utils.stringToJSON>[0]) => {
      const ret = utils.stringToJSON(obj);
      this.cache.load = ret;
      return Promise.resolve(ret);
    };

    switch (this.type) {
      case 'json':
      case 'string-json':
      case 'string-yaml':
        return resolve(this.file);

      case 'buffer':
        return resolve(this.file.toString());

      case 'url':
        const resp = await fetch(utils.normalizeURL(this.file)).then(res => res.text());
        return resolve(resp);

      case 'path':
        // Load a local file
        if (!this.opts.enablePaths) {
          return Promise.reject(new Error('Use `opts.enablePaths` to enable accessing local files.'));
        }

        const contents = fs.readFileSync(this.file).toString();
        if (!contents.trim()) {
          return Promise.reject(new Error('No file contents found.'));
        }
        return resolve(contents);

      default:
        return Promise.reject(new Error('Could not load this file.'));
    }
  }

  /**
   * @private
   */
  static convertPostmanToOpenAPI(schema: any) {
    return postmanToOpenAPI(JSON.stringify(schema), undefined, { outputFormat: 'json', replaceVars: true }).then(JSON.parse);
  }

  /**
   * Bundle up the given API definition, resolving any external `$ref` pointers in the process.
   *
   */
  async bundle() {
    if (this.cache.bundle) return Promise.resolve(this.cache.bundle);

    return this.load()
      .then(schema => {
        // Though Postman collections don't support `$ref` pointers for us to bundle we'll still
        // upconvert it to an OpenAPI definition file so our returned dataset is always one of
        // those for a Postman dataset.
        if (utils.isPostman(schema)) {
          return OASNormalize.convertPostmanToOpenAPI(schema);
        }

        return schema;
      })
      .then(schema => openapiParser.bundle(schema))
      .then(bundle => {
        this.cache.bundle = bundle;
        return bundle;
      });
  }

  /**
   * Dereference the given API definition.
   *
   */
  async deref() {
    if (this.cache.deref) return Promise.resolve(this.cache.deref);

    return this.load()
      .then(schema => {
        // Though Postman collections don't support `$ref` pointers for us to dereference we'll
        // still upconvert it to an OpenAPI definition file so our returned dataset is always one
        // of those for a Postman dataset.
        if (utils.isPostman(schema)) {
          return OASNormalize.convertPostmanToOpenAPI(schema);
        }

        return schema;
      })
      .then(schema => openapiParser.dereference(schema))
      .then(dereferenced => {
        this.cache.deref = dereferenced;
        return dereferenced;
      });
  }

  /**
   * Validate, and potentially convert to OpenAPI, a given API definition.
   *
   */
  async validate(
    opts: {
      /**
       * Automatically convert the supplied API definition to the latest version of OpenAPI.
       */
      convertToLatest?: boolean;
      parser?: openapiParser.Options;
    } = { convertToLatest: false },
  ) {
    const convertToLatest = opts.convertToLatest;
    const parserOptions = opts.parser || {};
    if (!parserOptions.validate) {
      parserOptions.validate = {};
    }

    parserOptions.validate.colorizeErrors = this.opts.colorizeErrors;

    return this.load()
      .then(async schema => {
        if (!utils.isPostman(schema)) {
          return schema;
        }

        return OASNormalize.convertPostmanToOpenAPI(schema);
      })
      .then(async schema => {
        if (!utils.isSwagger(schema) && !utils.isOpenAPI(schema)) {
          return Promise.reject(new Error('The supplied API definition is unsupported.'));
        } else if (utils.isSwagger(schema)) {
          const baseVersion = parseInt(schema.swagger, 10);
          if (baseVersion === 1) {
            return Promise.reject(new Error('Swagger v1.2 is unsupported.'));
          }
        }

        /**
         * `openapiParser.validate()` dereferences schemas at the same time as validation and does
         * not give us an option to disable this. Since all we already have a dereferencing method
         * on this library and our `validate()` method here just needs to tell us if the definition
         * is valid or not we need to clone it before passing it over to `openapi-parser` so as to
         * not run into pass-by-reference problems.
         */
        const clonedSchema = JSON.parse(JSON.stringify(schema));

        return openapiParser
          .validate(clonedSchema, parserOptions)
          .then(() => {
            if (!convertToLatest) {
              return schema;
            }

            return converter
              .convertObj(schema, { anchors: true })
              .then((options: { openapi: OpenAPI.Document }) => options.openapi);
          })
          .catch(err => Promise.reject(err));
      });
  }

  /**
   * Retrieve OpenAPI, Swagger, or Postman version information about the supplied API definition.
   *
   */
  version() {
    return this.load().then(schema => {
      switch (utils.getAPIDefinitionType(schema)) {
        case 'openapi':
          return {
            specification: 'openapi',
            version: schema.openapi,
          };

        case 'postman':
          let version = 'unknown';
          if (schema?.info?.schema) {
            // Though `info.schema` is required by the Postman spec there's no strictness to its
            // contents so we'll do our best to extract a version out of this schema URL that they
            // seem to usually match. If not we'll fallback to treating it as an `unknown` version.
            const match = schema.info.schema.match(
              /http(s?):\/\/schema.getpostman.com\/json\/collection\/v([0-9.]+)\//,
            );

            if (match) {
              version = match[2];
            }
          }

          return {
            specification: 'postman',
            version,
          };

        case 'swagger':
          return {
            specification: 'swagger',
            version: schema.swagger,
          };

        default:
          throw new Error('Unknown file detected.');
      }
    });
  }
}
