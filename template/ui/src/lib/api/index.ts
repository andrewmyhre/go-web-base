import { createApiClient } from '@andrewmyhre/web-base-ui/api';
import type { paths } from './openapi';

export type { paths };

/**
 * The API client, typed by this project's generated `paths`. Passing `paths`
 * makes the type checker prove the OpenAPI document still has the `GET /me`
 * and `POST /auth/logout` that the shared helpers (`getMe`, `signOut`) call.
 */
export const api = createApiClient<paths>();
