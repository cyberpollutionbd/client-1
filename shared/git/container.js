// @flow
import Git from '.'
import * as Constants from '../constants/git'
import * as Creators from '../actions/git/creators'
import {compose, lifecycle} from 'recompose'
import {connect} from 'react-redux'
import {createSelector} from 'reselect'
import partition from 'lodash/partition'

import type {TypedState} from '../constants/reducer'

const getIdToGit = (state: TypedState) => state.entities.getIn(['git', 'idToInfo'])

// sort by teamname then name
const sortRepos = (a: Constants.GitInfoRecord, b: Constants.GitInfoRecord) => {
  if (a.teamname) {
    if (b.teamname) {
      if (a.teamname === b.teamname) {
        return a.name.localeCompare(b.name)
      } else {
        return a.teamname.localeCompare(b.teamname)
      }
    }
    return -1
  }

  if (b.teamname) {
    return 1
  }

  return a.name.localeCompare(b.name)
}

const getRepos = createSelector([getIdToGit], git => {
  const [personals, teams] = partition(
    git.valueSeq().toArray().sort(sortRepos).map(g => g.repoID), // TODO uniqueid
    g => !g.teamname
  )

  return {
    personals,
    teams,
  }
})

const mapStateToProps = (state: TypedState, {routeState}) => {
  return {
    ...getRepos(state),
    expandedSet: routeState.expandedSet,
    loading: state.entities.getIn(['git', 'loading']),
  }
}

const mapDispatchToProps = (dispatch: any, {navigateAppend, setRouteState, routeState}) => ({
  _loadGit: () => dispatch(Creators.loadGit()),
  onNewPersonalRepo: () => dispatch(navigateAppend([{props: {isTeam: false}, selected: 'newRepo'}])),
  onNewTeamRepo: () => dispatch(navigateAppend([{props: {isTeam: true}, selected: 'newRepo'}])),
  onShowDelete: (id: string) => dispatch(navigateAppend([{props: {id}, selected: 'deleteRepo'}])),
  onToggleExpand: (id: string) => {
    const old = routeState.expandedSet
    // TODO use unique id
    setRouteState({expandedSet: old.has(id) ? old.delete(id) : old.add(id)})
  },
})

export default compose(
  connect(mapStateToProps, mapDispatchToProps),
  lifecycle({
    componentWillMount: function() {
      this.props._loadGit()
    },
  })
)(Git)