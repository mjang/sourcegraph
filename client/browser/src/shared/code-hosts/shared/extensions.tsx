import { createPlatformContext, SourcegraphIntegrationURLs, BrowserPlatformContext } from '../../platform/context'

import { CodeHost } from './codeHost'

/**
 * Initializes extensions for a page. It creates the {@link PlatformContext} and extensions controller.
 *
 */
export function initializeExtensions(
    { urlToFile }: Pick<CodeHost, 'urlToFile'>,
    urls: SourcegraphIntegrationURLs,
    isExtension: boolean
): { platformContext: BrowserPlatformContext } {
    const platformContext = createPlatformContext({ urlToFile }, urls, isExtension)
    return { platformContext }
}
