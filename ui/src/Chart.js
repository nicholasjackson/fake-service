import { Chart } from "react-google-charts";
import React from 'react'

class Timeline extends React.Component {

  constructor(props) {
    super(props);
    this.state = {
      url: props.url,
    };
  }

  componentDidMount() {
    this.fetchData();
  }

  fetchData() {
    fetch(this.state.url)
      .then(res => res.json())
      .then(
        (result) => {
          console.log("response from API:", result);

          // build the data
          var data = [
            [
              { type: 'string', id: 'Term' },
              { type: 'string', id: 'Name' },
              { type: 'date', id: 'Start' },
              { type: 'date', id: 'End' },
            ]
          ];

          // add the data from the nodes

          // add the root node
          console.log("date", Date.parse(result.start_time));
          data.push([1, result.name, Date.parse(result.start_time), Date.parse(result.end_time)]);

          this.setState({
            ...this.state,
            data: data,
          });
        },
        (error) => {
          console.error("error processing API", error);
        }
      );
  }

  render() {
    return <Chart
      width={'500px'}
      height={'300px'}
      chartType="Timeline"
      loader={<div>Loading Chart</div>}
      data={this.state.data}
      rootProps={{ 'data-testid': '1' }}
    />
  }
}

export default Timeline