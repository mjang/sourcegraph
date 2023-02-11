import React from 'react'

import { mdiHelpCircleOutline } from '@mdi/js'
import classNames from 'classnames'

import { Button, ButtonLink, ButtonLinkProps, Icon, Tooltip } from '@sourcegraph/wildcard'

import { TelemetryProps } from '../telemetry/telemetryService'

export interface HoverAction {
    title: string | '?'
    url?: string
    onClick?: () => void
    tooltip?: string
    disabled?: boolean
}

export interface ActionItemStyleProps {
    actionItemVariant?: ButtonLinkProps['variant']
    actionItemSize?: ButtonLinkProps['size']
    actionItemOutline?: ButtonLinkProps['outline']
}

export interface ActionItemComponentProps {
    iconClassName?: string
    actionItemStyleProps?: ActionItemStyleProps
}

// TODO(sqs): telemetry?
export const ActionItem: React.FunctionComponent<
    { action: HoverAction; className?: string } & ActionItemComponentProps & TelemetryProps
> = ({ action, actionItemStyleProps, className }) => {
    const content = action.title === '?' ? <Icon aria-hidden={true} svgPath={mdiHelpCircleOutline} /> : action.title
    const buttonProps: Pick<
        React.ComponentProps<typeof ButtonLink>,
        'aria-disabled' | 'className' | 'variant' | 'size' | 'outline' | 'disabled'
    > = {
        'aria-disabled': action.disabled,
        disabled: action.disabled,
        className: classNames('test-action-item', className),
        variant: actionItemStyleProps?.actionItemVariant,
        size: actionItemStyleProps?.actionItemSize,
        outline: actionItemStyleProps?.actionItemOutline,
    }

    return (
        <Tooltip content={action.tooltip}>
            {action.url ? (
                <ButtonLink {...buttonProps} to={action.url}>
                    {content}
                </ButtonLink>
            ) : action.onClick ? (
                <Button {...buttonProps} onClick={action.onClick}>
                    {content}
                </Button>
            ) : (
                <span className={className}>{content}</span>
            )}
        </Tooltip>
    )
}
