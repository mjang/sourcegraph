import { of } from 'rxjs'

import { MarkupKind } from '@sourcegraph/extension-api-classes'

import { PlatformContext } from '../platform/context'
import { EMPTY_SETTINGS_CASCADE, SettingsCascadeProps } from '../settings/settings'
import { NOOP_TELEMETRY_SERVICE } from '../telemetry/telemetryService'

import { HoverOverlayProps } from './HoverOverlay'

const NOOP_PLATFORM_CONTEXT: Pick<PlatformContext, 'settings'> = {
    settings: of({ final: {}, subjects: [] }),
}

export const commonProps = (): HoverOverlayProps & SettingsCascadeProps => ({
    telemetryService: NOOP_TELEMETRY_SERVICE,
    platformContext: NOOP_PLATFORM_CONTEXT,
    isLightTheme: true,
    overlayPosition: { top: 16, left: 16 },
    onAlertDismissed: action('onAlertDismissed'),
    settingsCascade: EMPTY_SETTINGS_CASCADE,
})

export const FIXTURE_CONTENT: Badged<MarkupContent> = {
    value:
        '```go\nfunc RegisterMiddlewares(m ...*Middleware)\n```\n\n' +
        '---\n\nRegisterMiddlewares registers additional authentication middlewares. Currently this is used to register enterprise-only SSO middleware. This should only be called from an init function.\n',
    kind: MarkupKind.Markdown,
}

export const FIXTURE_SEMANTIC_BADGE: AggregableBadge = {
    text: 'semantic',
    linkURL: 'https://docs.sourcegraph.com/code_navigation/explanations/precise_code_navigation',
    hoverMessage: 'Sample hover message',
}
