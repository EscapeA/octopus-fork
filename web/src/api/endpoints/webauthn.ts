import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../client';
import { useAuthStore } from './user';
import { logger } from '@/lib/logger';

/**
 * Passkey(WebAuthn)登录与凭证管理前端接口。
 *
 * 浏览器 WebAuthn API 使用 ArrayBuffer，而服务端(go-webauthn)使用 base64url 字符串，
 * 因此 begin 阶段需要把服务端选项里的 base64url 解码为 Buffer，finish 阶段需要把
 * 浏览器返回的 ArrayBuffer 编码为 base64url。
 */

// --- 公共状态 ---

export interface WebAuthnStatus {
    enabled: boolean;
    has_credentials: boolean;
}

export function useWebAuthnStatus() {
    return useQuery({
        queryKey: ['webauthn', 'status'],
        queryFn: async () => apiClient.get<WebAuthnStatus>('/api/v1/webauthn/status', undefined, false),
        refetchInterval: false,
        staleTime: 60_000,
    });
}

// --- base64url 转换 ---

function base64urlToBuffer(base64url: string): ArrayBuffer {
    // base64url → base64，补齐 padding 后 atob。
    const base64 = base64url.replace(/-/g, '+').replace(/_/g, '/');
    const padded = base64 + '='.repeat((4 - (base64.length % 4)) % 4);
    const binary = atob(padded);
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
    return bytes.buffer;
}

function bufferToBase64url(buf: ArrayBuffer | Uint8Array): string {
    const bytes = buf instanceof Uint8Array ? buf : new Uint8Array(buf);
    let binary = '';
    for (let i = 0; i < bytes.byteLength; i++) binary += String.fromCharCode(bytes[i]);
    return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

// 把服务端 PublicKey 选项里的 base64url 字段解码为 ArrayBuffer，供 navigator API 使用。
function decodeCreationOptions(publicKey: Record<string, unknown>): PublicKeyCredentialCreationOptions {
    const decoded = { ...publicKey } as Record<string, unknown>;
    decoded.challenge = base64urlToBuffer(decoded.challenge as string);
    if (decoded.user && typeof decoded.user === 'object') {
        decoded.user = { ...(decoded.user as object), id: base64urlToBuffer((decoded.user as { id: string }).id) };
    }
    if (Array.isArray(decoded.excludeCredentials)) {
        decoded.excludeCredentials = (decoded.excludeCredentials as Array<Record<string, unknown>>).map((c) => ({
            ...c,
            id: base64urlToBuffer(c.id as string),
        }));
    }
    return decoded as unknown as PublicKeyCredentialCreationOptions;
}

function decodeRequestOptions(publicKey: Record<string, unknown>): PublicKeyCredentialRequestOptions {
    const decoded = { ...publicKey } as Record<string, unknown>;
    decoded.challenge = base64urlToBuffer(decoded.challenge as string);
    if (Array.isArray(decoded.allowCredentials)) {
        decoded.allowCredentials = (decoded.allowCredentials as Array<Record<string, unknown>>).map((c) => ({
            ...c,
            id: base64urlToBuffer(c.id as string),
        }));
    }
    return decoded as unknown as PublicKeyCredentialRequestOptions;
}

interface BeginResponse {
    session_token: string;
    options: { publicKey: Record<string, unknown> };
}

// --- 浏览器响应序列化（→ 服务端 JSON）---

function serializeCreation(cred: PublicKeyCredential): Record<string, unknown> {
    const response = cred.response as AuthenticatorAttestationResponse;
    let transports: string[] = [];
    if (typeof response.getTransports === 'function') {
        transports = response.getTransports();
    }
    return {
        id: cred.id,
        rawId: bufferToBase64url(cred.rawId),
        type: cred.type,
        authenticatorAttachment: cred.authenticatorAttachment,
        clientExtensionResults: cred.getClientExtensionResults(),
        response: {
            attestationObject: bufferToBase64url(response.attestationObject),
            clientDataJSON: bufferToBase64url(response.clientDataJSON),
            transports,
        },
    };
}

function serializeAssertion(cred: PublicKeyCredential): Record<string, unknown> {
    const response = cred.response as AuthenticatorAssertionResponse;
    const out: Record<string, unknown> = {
        id: cred.id,
        rawId: bufferToBase64url(cred.rawId),
        type: cred.type,
        authenticatorAttachment: cred.authenticatorAttachment,
        clientExtensionResults: cred.getClientExtensionResults(),
        response: {
            authenticatorData: bufferToBase64url(response.authenticatorData),
            clientDataJSON: bufferToBase64url(response.clientDataJSON),
            signature: bufferToBase64url(response.signature),
        },
    };
    if (response.userHandle) {
        (out.response as Record<string, unknown>).userHandle = bufferToBase64url(response.userHandle);
    }
    return out;
}

export function isWebAuthnSupported(): boolean {
    return typeof window !== 'undefined'
        && typeof window.PublicKeyCredential !== 'undefined'
        && typeof navigator.credentials !== 'undefined';
}

/**
 * WebAuthn 要求安全上下文（HTTPS 或 localhost）。
 * 通过 http://IP:port 访问时 navigator.credentials.create() 会抛 SecurityError。
 */
export function isSecureContextForWebAuthn(): boolean {
    return typeof window !== 'undefined' && window.isSecureContext;
}

/**
 * classifyCredentialError 把 navigator.credentials.create/get 抛出的异常
 * 转换为用户可读的错误信息。手机端常抛 UnknownError（"an unknown error
 * occurred while talking to the credential manager"），根因多为 RP ID
 * 与访问域名不匹配或 origins 缺失，这里给出可操作的提示。
 */
function classifyCredentialError(e: unknown): Error {
    if (!(e instanceof DOMException) && !(e instanceof Error)) {
        return new Error(String(e));
    }
    const errName = e instanceof DOMException ? e.name : '';
    const msg = e.message || '';

    if (errName === 'SecurityError') {
        return new Error(
            'Browser blocked passkey operation. Ensure HTTPS and RP ID match the current domain.'
        );
    }
    if (errName === 'NotAllowedError') {
        return new Error('Passkey operation was cancelled or timed out.');
    }
    if (errName === 'AbortError') {
        return new Error('Passkey operation was aborted.');
    }
    if (errName === 'UnknownError' || /unknown error|credential manager/i.test(msg)) {
        return new Error(
            'The credential manager returned an unknown error. This usually means ' +
            'the WebAuthn RP ID or Origins setting does not match the current access domain. ' +
            'Check that RP ID is the current domain (or parent) and Origins includes this URL.'
        );
    }
    return new Error(msg || `Passkey operation failed (${errName || 'Unknown'})`);
}

// --- 登录（公开）---

export interface LoginResponse {
    token: string;
    expire_at: string;
}

export function usePasskeyLogin() {
    const { setAuth } = useAuthStore();
    return useMutation({
        mutationFn: async (): Promise<LoginResponse> => {
            const begin = await apiClient.post<BeginResponse>('/api/v1/webauthn/login/begin', {}, undefined, false);
            const publicKey = decodeRequestOptions(begin.options.publicKey);
            let credential: PublicKeyCredential | null;
            try {
                credential = await navigator.credentials.get({ publicKey }) as PublicKeyCredential | null;
            } catch (e) {
                throw classifyCredentialError(e);
            }
            if (!credential) throw new Error('Passkey login cancelled');
            return apiClient.post<LoginResponse>(
                '/api/v1/webauthn/login/finish',
                {
                    session_token: begin.session_token,
                    credential: serializeAssertion(credential as PublicKeyCredential),
                    // Passkey 是持久免密凭证，按"记住我"签发长效 token，避免短期内被强制退出。
                    expire: -1,
                },
                undefined,
                false,
            );
        },
        onSuccess: (data) => setAuth(data.token, data.expire_at),
        onError: (error) => logger.error('Passkey login failed:', error),
    });
}

// --- 凭证管理（鉴权）---

export interface WebAuthnCredentialView {
    id: number;
    name: string;
    created_at: string;
    last_used_at: string | null;
}

export function useWebAuthnCredentials() {
    return useQuery({
        queryKey: ['webauthn', 'credentials'],
        queryFn: async () => apiClient.get<WebAuthnCredentialView[]>('/api/v1/webauthn/credentials'),
        refetchInterval: false,
    });
}

export function useRegisterPasskey() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: async (name: string): Promise<void> => {
            if (!isSecureContextForWebAuthn()) {
                throw new Error('Passkey requires a secure context (HTTPS or localhost).');
            }
            const begin = await apiClient.post<BeginResponse>('/api/v1/webauthn/register/begin', { name });
            const publicKey = decodeCreationOptions(begin.options.publicKey);

            // 预检：RP ID 必须是当前域名的精确匹配或父域，否则手机端
            // credential manager 会抛 "unknown error talking to credential manager"。
            const rpId = (begin.options.publicKey as Record<string, unknown>)?.rp as { id?: string } | undefined;
            if (rpId?.id && typeof window !== 'undefined') {
                const hostname = window.location.hostname;
                if (hostname !== rpId.id && !hostname.endsWith('.' + rpId.id)) {
                    throw new Error(
                        `RP ID "${rpId.id}" does not match the current domain "${hostname}". ` +
                        'Update the WebAuthn RP ID setting to match your access domain.'
                    );
                }
            }

            let credential: PublicKeyCredential | null;
            try {
                credential = await navigator.credentials.create({ publicKey }) as PublicKeyCredential | null;
            } catch (e) {
                throw classifyCredentialError(e);
            }
            if (!credential) throw new Error('Passkey registration cancelled');
            await apiClient.post('/api/v1/webauthn/register/finish', {
                session_token: begin.session_token,
                name,
                credential: serializeCreation(credential),
            });
        },
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ['webauthn', 'credentials'] }),
        onError: (error) => logger.error('Passkey registration failed:', error),
    });
}

export function useDeletePasskey() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: async (id: number) => apiClient.delete(`/api/v1/webauthn/credentials/${id}`),
        onSuccess: () => queryClient.invalidateQueries({ queryKey: ['webauthn', 'credentials'] }),
        onError: (error) => logger.error('Passkey deletion failed:', error),
    });
}
