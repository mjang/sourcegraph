import React, { useEffect } from 'react'

import classNames from 'classnames'
import { Observable } from 'rxjs'

import { isErrorLike } from '@sourcegraph/common'
import { HoverOverlay, HoverOverlayProps } from '@sourcegraph/shared/src/hover/HoverOverlay'
import { Settings, SettingsCascadeOrError, SettingsCascadeProps } from '@sourcegraph/shared/src/settings/settings'

import { HoverThresholdProps } from '../../repo/RepoContainer'

import styles from './WebHoverOverlay.module.scss'

export interface WebHoverOverlayProps
    extends Omit<
            HoverOverlayProps,
            'className' | 'closeButtonClassName' | 'actionItemClassName' | 'actionItemStyleProps'
        >,
        HoverThresholdProps,
        SettingsCascadeProps {
    hoveredTokenElement?: HTMLElement
    /**
     * If the hovered token doesn't have a corresponding DOM element, this prop
     * can be used to trigger the "click to go to definition" functionality.
     */
    hoveredTokenClick?: Observable<unknown>
    nav?: (url: string) => void

    hoverOverlayContainerClassName?: string
}

export const WebHoverOverlay: React.FunctionComponent<React.PropsWithChildren<WebHoverOverlayProps>> = props => {
    const propsToUse = props

    const { hoverOrError } = propsToUse
    const { onHoverShown, hoveredToken } = props

    /** Whether the hover has actual content (that provides value to the user) */
    const hoverHasValue = hoverOrError !== 'loading' && !isErrorLike(hoverOrError) && !!hoverOrError?.contents?.length

    useEffect(() => {
        if (hoverHasValue) {
            onHoverShown?.()
        }
    }, [hoveredToken?.filePath, hoveredToken?.line, hoveredToken?.character, onHoverShown, hoverHasValue])

    return (
        <HoverOverlay
            {...propsToUse}
            className={classNames(styles.webHoverOverlay, props.hoverOverlayContainerClassName)}
            closeButtonClassName={styles.webHoverOverlayCloseButton}
            actionItemClassName="border-0"
            actionItemStyleProps={{
                actionItemSize: 'sm',
                actionItemVariant: 'secondary',
            }}
        />
    )
}

WebHoverOverlay.displayName = 'WebHoverOverlay'

export const getClickToGoToDefinition = (settingsCascade: SettingsCascadeOrError<Settings>): boolean => {
    if (settingsCascade.final && !isErrorLike(settingsCascade.final)) {
        const value = settingsCascade.final['codeIntelligence.clickToGoToDefinition'] as boolean
        return value ?? true
    }
    return true
}
