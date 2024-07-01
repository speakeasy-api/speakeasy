import { expect, test } from "vitest";
import { generateCodeSamplesOverlay } from "../src/generateCodeSamplesOverlay";

const coreTargets = [
  "shell",
  "javascript",
  "python",
  "go",
  "ruby",
  "php",
  "java",
  "csharp",
  "swift",
  "kotlin",
] as const;

test("request body example", async () => {
  const petstoreWithExamples = {
    openapi: "3.0.0",
    info: {
      title: "Swagger Petstore",
      version: "1.0.0",
    },
    paths: {
      "/pets": {
        post: {
          requestBody: {
            content: {
              "application/json": {
                schema: {
                  type: "object",
                  properties: {
                    name: {
                      type: "string",
                    },
                    breed: {
                      type: "string",
                    },
                  },
                  example: {
                    name: "doggie",
                    breed: "labrador",
                  },
                },
              },
            },
          },
          responses: {
            200: {
              description: "pet created",
            },
          },
        },
      },
    },
  };

  for (const target of coreTargets) {
    const res = await generateCodeSamplesOverlay(
      JSON.stringify(petstoreWithExamples),
      target
    );

    expect(res.errors).toBe(undefined);
    const resString = JSON.stringify(res.overlay, null, 2);

    expect(resString).toContain("doggie");
    expect(resString).toContain("labrador");
  }
});

test("request body no example", async () => {
  const petstoreWithExamples = {
    openapi: "3.0.0",
    info: {
      title: "Swagger Petstore",
      version: "1.0.0",
    },
    paths: {
      "/pets": {
        post: {
          requestBody: {
            content: {
              "application/json": {
                schema: {
                  type: "object",
                  properties: {
                    asdf: {
                      type: "string",
                    },
                  },
                },
              },
            },
          },
          responses: {
            200: {
              description: "pet created",
            },
          },
        },
      },
    },
  };

  for (const target of coreTargets) {
    const res = await generateCodeSamplesOverlay(
      JSON.stringify(petstoreWithExamples),
      target
    );
    expect(res.errors).toBe(undefined);
    const resString = JSON.stringify(res.overlay, null, 2);
    expect(resString).toContain("asdf");
  }
});

test("path parameter example", async () => {
  const petstoreWithExamples = {
    openapi: "3.0.0",
    info: {
      title: "Swagger Petstore",
      version: "1.0.0",
    },
    paths: {
      "/pets/{petId}": {
        get: {
          parameters: [
            {
              name: "petId",
              in: "path",
              required: true,
              schema: {
                type: "string",
              },
              example: "123",
            },
          ],
          responses: {
            200: {
              description: "pet found",
            },
          },
        },
      },
    },
  };

  for (const target of coreTargets) {
    const res = await generateCodeSamplesOverlay(
      JSON.stringify(petstoreWithExamples),
      target
    );

    expect(res.errors).toBe(undefined);
    const resString = JSON.stringify(res.overlay, null, 2);

    expect(resString).toContain("123");
  }
});

test("query parameter example", async () => {
  const petstoreWithExamples = {
    openapi: "3.0.0",
    info: {
      title: "Swagger Petstore",
      version: "1.0.0",
    },
    paths: {
      "/pets": {
        get: {
          parameters: [
            {
              name: "limit",
              in: "query",
              required: false,
              schema: {
                type: "string",
              },
              example: "asdf",
            },
          ],
          responses: {
            200: {
              description: "pet found",
            },
          },
        },
      },
    },
  };

  for (const target of coreTargets) {
    const res = await generateCodeSamplesOverlay(
      JSON.stringify(petstoreWithExamples),
      target
    );

    expect(res.errors).toBe(undefined);
    const resString = JSON.stringify(res.overlay, null, 2);

    expect(resString).toContain("asdf");
  }
});

test("request body component example", async () => {
  const petstoreWithExamples = {
    openapi: "3.0.0",
    info: {
      title: "Swagger Petstore",
      version: "1.0.0",
    },
    components: {
      schemas: {
        Pet: {
          type: "object",
          properties: {
            name: {
              type: "string",
            },
            breed: {
              type: "string",
            },
          },
          example: {
            name: "doggie",
            breed: "labrador",
          },
        },
      },
    },
    paths: {
      "/pets": {
        post: {
          requestBody: {
            content: {
              "application/json": {
                schema: {
                  $ref: "#/components/schemas/Pet",
                },
              },
            },
          },
          responses: {
            200: {
              description: "pet created",
            },
          },
        },
      },
    },
  };

  for (const target of coreTargets) {
    const res = await generateCodeSamplesOverlay(
      JSON.stringify(petstoreWithExamples),
      target
    );

    expect(res.errors).toBe(undefined);
    const resString = JSON.stringify(res.overlay, null, 2);

    expect(resString).toContain("doggie");
    expect(resString).toContain("labrador");
  }
});

test("request body component example with ref", async () => {
  const petstoreWithExamples = {
    openapi: "3.0.0",
    info: {
      title: "Swagger Petstore",
      version: "1.0.0",
    },
    components: {
      schemas: {
        Pet: {
          type: "object",
          properties: {
            name: {
              type: "string",
            },
            breed: {
              type: "string",
            },
          },
          example: {
            name: "doggie",
            breed: "labrador",
          },
        },
      },
    },
    paths: {
      "/pets": {
        post: {
          requestBody: {
            content: {
              "application/json": {
                schema: {
                  $ref: "#/components/schemas/Pet",
                },
              },
            },
          },
          responses: {
            200: {
              description: "pet created",
            },
          },
        },
      },
    },
  };

  for (const target of coreTargets) {
    const res = await generateCodeSamplesOverlay(
      JSON.stringify(petstoreWithExamples),
      target
    );

    expect(res.errors).toBe(undefined);
    const resString = JSON.stringify(res.overlay, null, 2);

    expect(resString).toContain("doggie");
    expect(resString).toContain("labrador");
  }
});

test("deep request body object leaf node example", async () => {
  const petstoreWithExamples = {
    openapi: "3.0.0",
    info: {
      title: "Swagger Petstore",
      version: "1.0.0",
    },
    paths: {
      "/pets": {
        post: {
          requestBody: {
            content: {
              "application/json": {
                schema: {
                  type: "object",
                  properties: {
                    name: {
                      type: "string",
                    },
                    breed: {
                      type: "object",
                      properties: {
                        name: {
                          type: "string",
                        },
                      },
                      example: {
                        name: "labrador",
                      },
                    },
                  },
                },
              },
            },
          },
          responses: {
            200: {
              description: "pet created",
            },
          },
        },
      },
    },
  };

  for (const target of coreTargets) {
    const res = await generateCodeSamplesOverlay(
      JSON.stringify(petstoreWithExamples),
      target
    );

    expect(res.errors).toBe(undefined);
    const resString = JSON.stringify(res.overlay, null, 2);

    expect(resString).toContain("labrador");
  }
});

// Skipped as multiple examples not parsed correctly doesn't seem to work
test.skip("request body with multiple examples", async () => {
  const petstoreWithExamples = {
    openapi: "3.0.0",
    info: {
      title: "Swagger Petstore",
      version: "1.0.0",
    },
    paths: {
      "/pets": {
        post: {
          requestBody: {
            content: {
              "application/json": {
                schema: {
                  type: "object",
                  properties: {
                    name: {
                      type: "string",
                    },
                    breed: {
                      type: "string",
                    },
                  },
                  examples: {
                    doggie: {
                      value: {
                        name: "doggie",
                        breed: "labrador",
                      },
                    },
                    kitty: {
                      value: {
                        name: "kitty",
                        breed: "siamese",
                      },
                    },
                  },
                },
              },
            },
          },
          responses: {
            200: {
              description: "pet created",
            },
          },
        },
      },
    },
  };

  for (const target of coreTargets) {
    const res = await generateCodeSamplesOverlay(
      JSON.stringify(petstoreWithExamples),
      target
    );

    expect(res.errors).toBe(undefined);
    const resString = JSON.stringify(res.overlay, null, 2);

    expect(resString).toContain("doggie");
    expect(resString).toContain("labrador");
    expect(resString).toContain("kitty");
    expect(resString).toContain("siamese");
  }
});

test("auth example", async () => {
  const petstoreWithAuth = {
    openapi: "3.0.0",
    info: {
      title: "Swagger Petstore",
      version: "1.0.0",
    },
    components: {
      securitySchemes: {
        apiKey: {
          type: "apiKey",
          in: "header",
          name: "X-API-Key",
        },
      },
    },
    paths: {
      "/pets": {
        get: {
          security: [
            {
              apiKey: [],
            },
          ],
          responses: {
            200: {
              description: "pet found",
            },
          },
        },
      },
    },
  };

  for (const target of coreTargets) {
    const res = await generateCodeSamplesOverlay(
      JSON.stringify(petstoreWithAuth),
      target
    );

    expect(res.errors).toBe(undefined);
    const resString = JSON.stringify(res.overlay, null, 2);

    expect(resString).toContain("X-API-Key");
  }
});

test.todo("union example");
test.todo("inline example for ref");
test.todo("media types");
