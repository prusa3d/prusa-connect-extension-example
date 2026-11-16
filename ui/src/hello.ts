// The service layer: this module's own route, reached through the client the
// host handed over.
//
// `bind` comes from `hello-client`, which `make client` generates from
// openapi.json. It attaches one method per operation onto the transport of the
// host's client, so a call carries the person's session and goes to
// /api/v1/modules/<this module's namespace>/hello. Nothing here names a URL, a
// header or a token: a module's browser half never holds a credential.
import type { ModuleHost } from '@prusa/connect-ui/slots';
import { bind, type operations } from 'hello-client';

/** What GET /hello answers, as openapi.json declares it. */
export type Hello =
  operations['read-hello']['responses'][200]['content']['application/json'];

/** The route, bound onto the host's client. */
export const helloOn = (host: ModuleHost) => {
  const routes = bind(host.client);

  return {
    read: (signal?: AbortSignal): Promise<Hello> =>
      routes.readHello({}, signal),
  };
};
