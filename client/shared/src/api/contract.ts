import { Subscribable } from 'rxjs'

import { TextDocumentPositionParameters, HoverMerged } from '@sourcegraph/client-api'
import { MaybeLoadingResult } from '@sourcegraph/codeintellify'
import * as clientType from '@sourcegraph/extension-api-types'

import { DocumentHighlight, ReferenceContext } from '../codeintel/legacy-extensions/api'

/**
 * This is exposed from the extension host thread to the main thread
 * e.g. for communicating  direction "main -> ext host"
 * Note this API object lives in the extension host thread
 */
export interface FlatExtensionHostAPI {
    // Languages
    getHover: (parameters: TextDocumentPositionParameters) => Subscribable<MaybeLoadingResult<HoverMerged | null>>
    getDocumentHighlights: (parameters: TextDocumentPositionParameters) => Subscribable<DocumentHighlight[]>
    getDefinition: (
        parameters: TextDocumentPositionParameters
    ) => Subscribable<MaybeLoadingResult<clientType.Location[]>>
    getReferences: (
        parameters: TextDocumentPositionParameters,
        context: ReferenceContext
    ) => Subscribable<MaybeLoadingResult<clientType.Location[]>>
    getLocations: (
        id: string,
        parameters: TextDocumentPositionParameters
    ) => Subscribable<MaybeLoadingResult<clientType.Location[]>>

    hasReferenceProvidersForDocument: (parameters: TextDocumentPositionParameters) => Subscribable<boolean>
}
