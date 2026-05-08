/**
 * Minimal placeholder — overwritten by `npm run types`.
 * Covers the auth endpoints needed for Phase 1.
 */
export interface paths {
  '/auth/login': {
    post: {
      requestBody: {
        content: { 'application/json': { username: string; password: string } }
      }
      responses: {
        200: { content: { 'application/json': { access_token: string; expires_in: number } } }
        400: { content: { 'application/json': { error: { code: string; message: string } } } }
        401: { content: { 'application/json': { error: { code: string; message: string } } } }
      }
    }
  }
  '/auth/logout': {
    post: {
      requestBody?: never
      responses: { 204: { content?: never } }
    }
  }
  '/auth/refresh': {
    post: {
      requestBody?: never
      responses: {
        200: { content: { 'application/json': { access_token: string; expires_in: number } } }
        401: { content: { 'application/json': { error: { code: string; message: string } } } }
      }
    }
  }
}
