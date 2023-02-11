import { EditorView, Tooltip, TooltipView } from '@codemirror/view'
import { concat, from, Observable, of } from 'rxjs'
import { map } from 'rxjs/operators'

import { HoverMerged } from '@sourcegraph/client-api'
import { MarkupKind } from '@sourcegraph/extension-api-classes'
import { HoverAction } from '@sourcegraph/shared/src/actions/ActionItem'
import { hasFindImplementationsSupport } from '@sourcegraph/shared/src/codeintel/api'
import { Occurrence } from '@sourcegraph/shared/src/codeintel/scip'
import { BlobViewState, toPrettyBlobURL } from '@sourcegraph/shared/src/util/url'

import { blobPropsFacet } from '..'
import { HovercardView, HoverData } from '../hovercard'
import { rangeToCmSelection } from '../occurrence-utils'
import { DefinitionResult, goToDefinitionAtOccurrence } from '../token-selection/definition'
import { modifierClickDescription } from '../token-selection/modifier-click'

export interface HoverResult {
    markdownContents: string
    hoverMerged?: HoverMerged | null
    isPrecise?: boolean
}
export const emptyHoverResult: HoverResult = { markdownContents: '' }

// Helper to handle the cases for "No definition found" and "You are at the definition".
interface AsyncDefinitionResult extends DefinitionResult {
    asyncHandler: () => Promise<void>
}

// CodeMirror tooltip wrapper around the "code intel" popover.  Implemented as a
// class so that we can detect it with instanceof checks. This class
// reimplements logic from `getHoverActions` in
// 'client/shared/src/hover/actions.ts' because that function is difficult to
// reason about and has surprising behavior.
export class CodeIntelTooltip implements Tooltip {
    public readonly above = true
    public readonly pos: number
    public readonly end: number
    public readonly create: () => TooltipView
    constructor(
        private readonly view: EditorView,
        private readonly occurrence: Occurrence,
        private readonly hover: HoverResult,
        // eslint-disable-next-line @typescript-eslint/explicit-member-accessibility
        readonly pinned: boolean
    ) {
        const range = rangeToCmSelection(view.state, occurrence.range)
        this.pos = range.from
        this.end = range.to
        this.create = () => {
            // To prevent the "Go to definition" from delaying the loading of
            // the popover, we provide an instant result that doesn't handle the
            // "No definition found" or "You are at the definition" cases. This
            // instant result gets dynamically replaced the actual result once
            // it finishes loading.
            const instantDefinitionResult: AsyncDefinitionResult = {
                locations: [{ uri: '' }],
                handler: () => {},
                asyncHandler: () =>
                    goToDefinitionAtOccurrence(view, occurrence).then(
                        ({ handler }) => handler(occurrence.range.start),
                        () => {}
                    ),
            }
            const definitionResults: Observable<AsyncDefinitionResult> = concat(
                // Show active "Go to definition" button until we have resolved
                // a definition.
                of(instantDefinitionResult),
                // Trigger "Go to definition" to identify if this hover message
                // is already at the definition or if there are no references.
                from(goToDefinitionAtOccurrence(view, occurrence)).pipe(
                    map(result => ({ ...result, asyncHandler: instantDefinitionResult.asyncHandler }))
                )
            )
            const hovercardData: Observable<HoverData> = definitionResults.pipe(
                map(result => this.hovercardData(result))
            )
            return new HovercardView(view, occurrence.range.withIncrementedValues(), pinned, hovercardData)
        }
    }
    private hovercardData(definition: AsyncDefinitionResult): HoverData {
        const { markdownContents } = this.hover
        const blobInfo = this.view.state.facet(blobPropsFacet).blobInfo
        const referencesURL = toPrettyBlobURL({
            ...blobInfo,
            range: this.occurrence.range.withIncrementedValues(),
            viewState: 'references',
        })
        const noDefinitionFound = definition.locations.length === 0
        const actions: HoverAction[] = [
            {
                title: noDefinitionFound
                    ? 'No definition found'
                    : definition.atTheDefinition
                    ? 'At definition'
                    : 'Go to definition',
                url: definition.url,
                onClick: definition.url ? undefined : () => definition.asyncHandler(),
                disabled: noDefinitionFound || definition.atTheDefinition,
            },
            {
                title: 'Find references',
                url: referencesURL,
            },
        ]
        if (
            this.hover.isPrecise &&
            hasFindImplementationsSupport(this.view.state.facet(blobPropsFacet).blobInfo.mode)
        ) {
            const implementationsURL = toPrettyBlobURL({
                ...blobInfo,
                range: this.occurrence.range.withIncrementedValues(),
                viewState: `implementations_${blobInfo.mode}` as BlobViewState,
            })
            actions.push({
                title: 'Find implementations',
                url: implementationsURL,
            })
        }
        actions.push({
            title: '?', // special marker for the MDI "Help" icon.
            tooltip: `Go to definition with ${modifierClickDescription}, long-click, or by pressing Enter with the keyboard. Display this popover by pressing Space with the keyboard.`,
        })
        return {
            actionsOrError: actions,
            hoverOrError: {
                range: this.occurrence.range,
                aggregatedBadges: this.hover.hoverMerged?.aggregatedBadges,
                contents: [
                    {
                        value: markdownContents,
                        kind: MarkupKind.Markdown,
                    },
                ],
            },
        }
    }
}
