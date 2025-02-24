import React, { useCallback, useRef, useMemo } from 'react'

import { useLocation, useNavigate } from 'react-router-dom-v5-compat'
import shallow from 'zustand/shallow'

import { SearchBox, Toggles } from '@sourcegraph/branded'
// The experimental search input should be shown in the navbar
// eslint-disable-next-line no-restricted-imports
import { LazyCodeMirrorQueryInput } from '@sourcegraph/branded/src/search-ui/experimental'
import { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'
import { SearchContextInputProps, SubmitSearchParameters } from '@sourcegraph/shared/src/search'
import { SettingsCascadeProps } from '@sourcegraph/shared/src/settings/settings'
import { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { ThemeProps } from '@sourcegraph/shared/src/theme'
import { Form } from '@sourcegraph/wildcard'

import { AuthenticatedUser } from '../../auth'
import { useExperimentalFeatures, useNavbarQueryState, setSearchCaseSensitivity } from '../../stores'
import { NavbarQueryState, setSearchMode, setSearchPatternType } from '../../stores/navbarSearchQueryState'

import { useLazyCreateSuggestions, useLazyHistoryExtension } from './lazy'
import { useRecentSearches } from './useRecentSearches'

interface Props
    extends SettingsCascadeProps,
        ThemeProps,
        SearchContextInputProps,
        TelemetryProps,
        PlatformContextProps<'requestGraphQL'> {
    authenticatedUser: AuthenticatedUser | null
    isSourcegraphDotCom: boolean
    globbing: boolean
    isSearchAutoFocusRequired?: boolean
    isRepositoryRelatedPage?: boolean
}

const selectQueryState = ({
    queryState,
    setQueryState,
    submitSearch,
    searchCaseSensitivity,
    searchPatternType,
    searchMode,
}: NavbarQueryState): Pick<
    NavbarQueryState,
    'queryState' | 'setQueryState' | 'submitSearch' | 'searchCaseSensitivity' | 'searchPatternType' | 'searchMode'
> => ({ queryState, setQueryState, submitSearch, searchCaseSensitivity, searchPatternType, searchMode })

/**
 * The search item in the navbar
 */
export const SearchNavbarItem: React.FunctionComponent<React.PropsWithChildren<Props>> = (props: Props) => {
    const navigate = useNavigate()
    const location = useLocation()

    const { queryState, setQueryState, submitSearch, searchCaseSensitivity, searchPatternType, searchMode } =
        useNavbarQueryState(selectQueryState, shallow)

    const applySuggestionsOnEnter =
        useExperimentalFeatures(features => features.applySearchQuerySuggestionOnEnter) ?? true
    const experimentalQueryInput = useExperimentalFeatures(features => features.searchQueryInput === 'experimental')

    const { recentSearches } = useRecentSearches()
    const recentSearchesRef = useRef(recentSearches)
    recentSearchesRef.current = recentSearches

    const submitSearchOnChange = useCallback(
        (parameters: Partial<SubmitSearchParameters> = {}) => {
            submitSearch({
                historyOrNavigate: navigate,
                location,
                source: 'nav',
                selectedSearchContextSpec: props.selectedSearchContextSpec,
                ...parameters,
            })
        },
        [submitSearch, navigate, location, props.selectedSearchContextSpec]
    )
    const submitSearchOnChangeRef = useRef(submitSearchOnChange)
    submitSearchOnChangeRef.current = submitSearchOnChange

    const onSubmit = useCallback(
        (event?: React.FormEvent): void => {
            event?.preventDefault()
            submitSearchOnChangeRef.current()
        },
        [submitSearchOnChangeRef]
    )

    const suggestionSource = useLazyCreateSuggestions(
        experimentalQueryInput,
        useMemo(
            () => ({
                platformContext: props.platformContext,
                authenticatedUser: props.authenticatedUser,
                fetchSearchContexts: props.fetchSearchContexts,
                getUserSearchContextNamespaces: props.getUserSearchContextNamespaces,
                isSourcegraphDotCom: props.isSourcegraphDotCom,
            }),
            [
                props.platformContext,
                props.authenticatedUser,
                props.fetchSearchContexts,
                props.getUserSearchContextNamespaces,
                props.isSourcegraphDotCom,
            ]
        )
    )

    const experimentalExtensions = useLazyHistoryExtension(
        experimentalQueryInput,
        recentSearchesRef,
        submitSearchOnChangeRef
    )

    if (experimentalQueryInput) {
        return (
            <Form
                className="search--navbar-item d-flex align-items-flex-start flex-grow-1 flex-shrink-past-contents"
                onSubmit={onSubmit}
            >
                <LazyCodeMirrorQueryInput
                    patternType={searchPatternType}
                    interpretComments={false}
                    queryState={queryState}
                    onChange={setQueryState}
                    onSubmit={onSubmit}
                    isLightTheme={props.isLightTheme}
                    placeholder="Search for code or files..."
                    suggestionSource={suggestionSource}
                    extensions={experimentalExtensions}
                >
                    <Toggles
                        patternType={searchPatternType}
                        caseSensitive={searchCaseSensitivity}
                        setPatternType={setSearchPatternType}
                        setCaseSensitivity={setSearchCaseSensitivity}
                        searchMode={searchMode}
                        setSearchMode={setSearchMode}
                        settingsCascade={props.settingsCascade}
                        navbarSearchQuery={queryState.query}
                    />
                </LazyCodeMirrorQueryInput>
            </Form>
        )
    }

    return (
        <Form
            className="search--navbar-item d-flex align-items-flex-start flex-grow-1 flex-shrink-past-contents"
            onSubmit={onSubmit}
        >
            <SearchBox
                {...props}
                autoFocus={false}
                applySuggestionsOnEnter={applySuggestionsOnEnter}
                showSearchContext={props.searchContextsEnabled}
                showSearchContextManagement={true}
                caseSensitive={searchCaseSensitivity}
                setCaseSensitivity={setSearchCaseSensitivity}
                patternType={searchPatternType}
                setPatternType={setSearchPatternType}
                searchMode={searchMode}
                setSearchMode={setSearchMode}
                queryState={queryState}
                onChange={setQueryState}
                onSubmit={onSubmit}
                submitSearchOnToggle={submitSearchOnChange}
                submitSearchOnSearchContextChange={submitSearchOnChange}
                isExternalServicesUserModeAll={window.context.externalServicesUserMode === 'all'}
                structuralSearchDisabled={window.context?.experimentalFeatures?.structuralSearch === 'disabled'}
                hideHelpButton={false}
                showSearchHistory={true}
                recentSearches={recentSearches}
            />
        </Form>
    )
}
